"""Thin native MemPalace facade; never import its stdio MCP server."""
import asyncio
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import re
import threading
import time

_ROOT = None
_PROCESS_GATE = threading.Lock()  # upstream palace lock is process-reentrant


class MemoryConflict(ValueError):
    pass


class MemoryIncomplete(RuntimeError):
    pass


def configure(root, *, offline=True):
    global _ROOT
    root = Path(root).resolve()
    if _ROOT is not None and _ROOT != root:
        raise RuntimeError('One isolated native-memory configuration per worker; reset to change it')
    _ROOT = root
    temporary = root / 'tmp'
    temporary.mkdir(parents=True, exist_ok=True)
    os.environ.update(MEMPALACE_CONFIG_DIR=str(root / 'config'), HF_HOME=str(root / 'cache'),
                      HF_HUB_CACHE=str(root / 'cache/hub'), MEMPALACE_EMBEDDING_MODEL='embeddinggemma',
                      MEMPALACE_EMBEDDING_DEVICE='cpu', MEMPALACE_EMBEDDING_THREADS='2',
                      ANONYMIZED_TELEMETRY='False', HF_HUB_OFFLINE='1' if offline else '0',
                      TMP=str(temporary), TEMP=str(temporary), SQLITE_TMPDIR=str(temporary))
    # Upstream CONFIG_DIR controls configuration, but NOT state/locks. Bind the
    # upstream path seam before importing palace, without changing the user's
    # home directory or vendoring a modified engine.
    import mempalace.paths as paths
    paths.state_home = lambda home=None: root / 'state'
    paths.state_str = lambda *parts: str((root / 'state').joinpath(*parts))
    return root


def prepare_assets(root, *, offline=False):
    """Explicit download and inference probe; normal operations require this manifest."""
    root = configure(root, offline=offline)
    from huggingface_hub import hf_hub_download
    from mempalace.embedding import _EMBEDDINGGEMMA_REPO, _EMBEDDINGGEMMA_ONNX, get_embedding_function
    revision = '5090578d9565bb06545b4552f76e6bc2c93e4a66'
    files = []
    for name in ['onnx/' + _EMBEDDINGGEMMA_ONNX, 'onnx/' + _EMBEDDINGGEMMA_ONNX + '_data', 'tokenizer.json']:
        path = Path(hf_hub_download(_EMBEDDINGGEMMA_REPO, filename=name, revision=revision, local_files_only=offline))
        with path.open('rb') as stream:
            digest = hashlib.file_digest(stream, 'sha256').hexdigest()
        files.append({'path': str(path), 'bytes': path.stat().st_size, 'sha256': digest})
    # The upstream embedding seam resolves the default revision. Bind that name
    # to this explicitly prepared immutable snapshot inside our isolated cache.
    reference = root / 'cache/hub' / ('models--' + _EMBEDDINGGEMMA_REPO.replace('/', '--')) / 'refs/main'
    reference.parent.mkdir(parents=True, exist_ok=True)
    reference.write_text(revision, encoding='utf-8')
    vector = get_embedding_function(model='embeddinggemma')(['工作台 preparation probe'])[0]
    manifest = {'model': 'embeddinggemma', 'repository': _EMBEDDINGGEMMA_REPO,
                'revision': revision, 'dimension': len(vector), 'files': files}
    root.mkdir(parents=True, exist_ok=True)
    temporary = root / 'embedding.json.tmp'
    temporary.write_text(json.dumps(manifest), encoding='utf-8')
    os.replace(temporary, root / 'embedding.json')
    return manifest


def probe(root):
    """Real engine readiness before environment publication, on synthetic data."""
    import uuid
    import shutil
    async def check():
        memory = Memory(root)
        memory.palace = str(memory.root / 'probes' / uuid.uuid4().hex)
        palace = memory.palace
        try:
            record = await memory.remember('Synthetic workbench preparation probe.', source_id='vivy-prepare', event_id=uuid.uuid4().hex)
            assert (await memory.get(record['id']))['content'] == 'Synthetic workbench preparation probe.'
            hits = await memory.search('Synthetic workbench preparation', source_id='vivy-prepare')
            assert any(hit['id'] == record['id'] for hit in hits), 'Semantic search did not return the probe'
            await memory.close()
            memory = Memory(root)
            memory.palace = palace
            assert (await memory.get(record['id']))['content'] == 'Synthetic workbench preparation probe.'
        finally:
            await memory.close()
            if Path(palace).exists():
                shutil.rmtree(palace, ignore_errors=False)
    asyncio.run(check())


class Memory:
    def __init__(self, root):
        self.root = configure(root)
        manifest = self.root / 'embedding.json'
        if not manifest.is_file():
            raise RuntimeError('Native memory assets missing; run /python prepare with python.memory=true')
        identity = json.loads(manifest.read_text(encoding='utf-8'))
        if identity['model'] != 'embeddinggemma':
            raise RuntimeError('Prepared embedding identity does not match embeddinggemma')
        for row in identity['files']:
            path = Path(row['path'])
            if not path.is_file() or path.stat().st_size != row['bytes']:
                raise RuntimeError('Prepared embedding assets are missing or incomplete; prepare again')
            with path.open('rb') as stream:
                if hashlib.file_digest(stream, 'sha256').hexdigest() != row['sha256']:
                    raise RuntimeError('Prepared embedding asset hash mismatch; prepare again')
        self.dimension = identity['dimension']
        self.palace = str(self.root / 'palace')
        self.executor = ThreadPoolExecutor(max_workers=1, thread_name_prefix='vivy-memory')
        self.closed = False

    async def _call(self, operation, *, write=False):
        if self.closed:
            raise RuntimeError('Native memory is closed; reset to reopen')
        future = asyncio.get_running_loop().run_in_executor(self.executor, self._locked, operation, write)
        # Cancelling an await cannot prove a synchronous write rolled back.
        # Keep ownership until shutdown; callers must inspect the same event ID.
        try:
            return await asyncio.shield(future)
        except asyncio.CancelledError:
            future.add_done_callback(lambda done: done.exception() if not done.cancelled() else None)
            raise

    def _locked(self, operation, write):
        from mempalace.palace import get_collection, mine_palace_lock, MineAlreadyRunning
        from mempalace.backends import get_backend
        from mempalace.embedding import get_embedder_identity
        deadline = time.monotonic() + 5
        if not _PROCESS_GATE.acquire(timeout=5):
            raise RuntimeError('Native memory busy; no write started')
        try:
            while True:
                lock = mine_palace_lock(self.palace)
                try:
                    lock.__enter__()
                    break
                except MineAlreadyRunning:
                    if time.monotonic() >= deadline:
                        raise RuntimeError('Native memory busy; no write started')
                    time.sleep(.05)
            try:
                identity = get_embedder_identity()
                if identity.dimension != self.dimension:
                    raise RuntimeError('Embedding dimension probe failed or disagrees with prepared identity; no write started')
                collection = get_collection(self.palace, backend='chroma', create=write, read_only=not write)
                stored = collection.get_stored_embedder_identity()
                if stored is not None and (stored.model_name != identity.model_name or stored.dimension not in (0, self.dimension)):
                    raise RuntimeError('Stored embedding model/dimension disagrees with prepared identity; no write started')
                if write and (stored is None or stored.dimension == 0):
                    collection.set_embedder_identity(identity)
                return operation(collection)
            finally:
                try:
                    get_backend('chroma').close_palace(self.palace)
                finally:
                    lock.__exit__(None, None, None)
        finally:
            _PROCESS_GATE.release()

    @staticmethod
    def _identity(source_id, event_id):
        if not isinstance(source_id, str) or not source_id or not isinstance(event_id, str) or not event_id:
            raise ValueError('Nonempty source_id and event_id are required')
        return 'vivy-' + hashlib.sha256(json.dumps([source_id, event_id], ensure_ascii=False).encode()).hexdigest()

    @staticmethod
    def _read(collection, identifier):
        if not re.fullmatch(r'vivy-[0-9a-f]{64}', identifier):
            raise ValueError('Invalid native memory logical ID')
        result = collection.get(where={'vivy_id': identifier}, include=['documents', 'metadatas'])
        if not result.ids:
            raise KeyError(identifier)
        rows = sorted(zip(result.metadatas, result.documents), key=lambda row: row[0]['chunk_index'])
        metadata = rows[0][0]
        count = metadata['chunk_count']
        if len(rows) != count or [m['chunk_index'] for m, _ in rows] != list(range(count)):
            raise MemoryIncomplete('Native memory chunk sequence is incomplete; do not replay the write')
        for row, _ in rows:
            for key in ['vivy_id', 'content_hash', 'chunk_count', 'source_id', 'event_id', 'wing', 'room']:
                if row.get(key) != metadata[key]:
                    raise MemoryIncomplete('Native memory chunk metadata disagrees')
        content = ''.join(text for _, text in rows)
        if hashlib.sha256(content.encode()).hexdigest() != metadata['content_hash']:
            raise MemoryIncomplete('Native memory content hash does not match')
        return {'id': identifier, 'content': content, 'source_id': metadata['source_id'],
                'event_id': metadata['event_id'], 'wing': metadata['wing'], 'room': metadata['room'], 'complete': True}

    async def remember(self, content, *, source_id, event_id, wing='workbench', room='events'):
        identifier = self._identity(source_id, event_id)
        if not isinstance(content, str) or not content or len(content.encode()) > 1024 * 1024:
            raise ValueError('Memory content must be nonempty text, at most 1 MiB; split larger input into explicit events')
        if not isinstance(wing, str) or not wing or not isinstance(room, str) or not room:
            raise ValueError('wing and room must be nonempty strings')
        def write(collection):
            try:
                existing = self._read(collection, identifier)
            except KeyError:
                existing = None
            if existing is not None:
                if any(existing[k] != v for k, v in [('content', content), ('wing', wing), ('room', room)]):
                    raise MemoryConflict('Same source/event has different content or partition; use a new event_id')
                return {'id': identifier, 'deduplicated': True, 'complete': True}
            chunks = [content[i:i + 2048] for i in range(0, len(content), 2048)]
            common = {'vivy_id': identifier, 'content_hash': hashlib.sha256(content.encode()).hexdigest(),
                      'source_id': source_id, 'event_id': event_id, 'source_file': source_id,
                      'wing': wing, 'room': room, 'chunk_count': len(chunks), 'parent_drawer_id': identifier}
            collection.upsert(ids=[f'{identifier}_chunk_{i:06d}' for i in range(len(chunks))], documents=chunks,
                              metadatas=[dict(common, chunk_index=i) for i in range(len(chunks))])
            self._read(collection, identifier)
            return {'id': identifier, 'deduplicated': False, 'complete': True}
        return await self._call(write, write=True)

    async def get(self, identifier=None, *, source_id=None, event_id=None):
        identifier = identifier or self._identity(source_id, event_id)
        return await self._call(lambda collection: self._read(collection, identifier))

    async def search(self, query, *, source_id=None, wing=None, room=None, limit=5):
        if not isinstance(query, str) or not query or not isinstance(limit, int) or not 1 <= limit <= 50:
            raise ValueError('Search needs nonempty query and limit between 1 and 50')
        def find(collection):
            filters = [{'vivy_id': {'$ne': ''}}]
            filters.extend({key: value} for key, value in [('source_id', source_id), ('wing', wing), ('room', room)] if value is not None)
            where = filters[0] if len(filters) == 1 else {'$and': filters}
            result = collection.query(query_texts=[query], n_results=limit, where=where, include=['metadatas', 'distances'])
            hits, seen = [], set()
            for metadata, distance in zip(result.metadatas[0], result.distances[0]):
                identifier = metadata['vivy_id']
                if identifier not in seen:
                    hits.append(dict(self._read(collection, identifier), distance=distance))
                    seen.add(identifier)
            return hits
        return await self._call(find)

    async def close(self):
        if self.closed:
            return
        self.closed = True
        # The host owns the hard deadline for a blocked native library.
        await asyncio.to_thread(self.executor.shutdown, wait=True, cancel_futures=True)

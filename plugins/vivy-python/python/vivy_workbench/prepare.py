"""Explicit uv preparation. Publish a pointer only after a fresh environment probes."""
from contextlib import contextmanager
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import uuid


@contextmanager
def environment_lock(root):
    root = Path(root)
    root.mkdir(parents=True, exist_ok=True)
    # Windows may refuse even the initial byte write while Rust holds a whole-
    # file lock. Normalize that path as the same competing-owner result.
    stream = None
    try:
        stream = (root / 'prepare.lock').open('a+b')
        if stream.tell() == 0:
            stream.write(b'\0')
            stream.flush()
        stream.seek(0)
    except OSError as exc:
        if stream:
            try:
                stream.close()
            except OSError:
                pass
        raise RuntimeError('Python preparation is busy in another process') from exc
    with stream:
        try:
            if os.name == 'nt':
                import msvcrt
                msvcrt.locking(stream.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl
                fcntl.flock(stream.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError as exc:
            raise RuntimeError('Python preparation is busy in another process') from exc
        try:
            yield
        finally:
            stream.seek(0)
            if os.name == 'nt':
                msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                fcntl.flock(stream.fileno(), fcntl.LOCK_UN)


def run(command, *, temporary=None, timeout=None):
    environment = dict(os.environ)
    # These overrides change integrity, target directories or constraints outside
    # the reviewed lock/Skill scope. Source/index overrides remain scope-hashed.
    for name in ('UV_NO_VERIFY_HASHES', 'UV_SYSTEM_PYTHON', 'UV_CONFIG_FILE', 'UV_PROJECT',
                 'UV_WORKING_DIR', 'UV_CONSTRAINT', 'UV_OVERRIDE', 'UV_EXCLUDE',
                 'UV_BUILD_CONSTRAINT', 'UV_TORCH_BACKEND', 'UV_NO_SOURCES'):
        environment.pop(name, None)
    environment['UV_NO_CONFIG'] = '1'
    environment['UV_PYTHON_DOWNLOADS'] = 'never'
    for name in list(environment):
        if name.upper().startswith('RIG_SECRET_'):
            del environment[name]
    if temporary is not None:
        environment.update(TMP=str(temporary), TEMP=str(temporary), SQLITE_TMPDIR=str(temporary))
        environment['UV_CACHE_DIR'] = str(Path(temporary).parent / 'uv-cache')
    # A file prevents unbounded installer transcripts from filling host memory.
    with tempfile.TemporaryFile(dir=temporary) as output:
        result = subprocess.run(command, env=environment, stdout=output, stderr=subprocess.STDOUT, timeout=timeout)
        output.seek(0, 2)
        output.seek(max(0, output.tell() - 16384))
        tail = output.read().decode('utf-8', errors='replace')
    if result.returncode:
        tail = re.sub(r'https?://\S+', '[URL redacted]', tail)
        for key, value in environment.items():
            if len(value) >= 4 and any(word in key.upper() for word in ('KEY', 'TOKEN', 'SECRET', 'PASSWORD', 'CREDENTIAL')):
                tail = tail.replace(value, '[redacted]')
        raise RuntimeError(f'Python preparation failed at {command[1]} (exit {result.returncode}); existing environment preserved\n{tail}')
    return tail


def stage(name):
    print(json.dumps({'type': 'stage', 'stage': name}), flush=True)


def skill_probe(skills):
    return ('import sys;sys.path.insert(0,' + repr(str(Path(__file__).resolve().parents[1])) + ');'
            'from vivy_workbench.capabilities import load;'
            'rows=load(' + repr(list(skills)) + ',{});'
            'errors=[r for r in rows if r["state"]=="unavailable"];'
            'assert not errors, errors')


def check(root, *, scope='', native_memory=False, memory_root=None):
    """Bounded, local detection only: no installation or database writes."""
    published = Path(root) / 'current.json'
    if not published.is_file():
        return {'outcome': 'not_init', 'detail': 'Python environment has not been prepared'}
    try:
        current = json.loads(published.read_text(encoding='utf-8'))
        executable = current['executable']
        if not Path(executable).is_file():
            raise RuntimeError('Prepared Python executable is missing')
        # Never import Skills from a publication after trust/dependency scope changes.
        if scope and current.get('scope') != scope:
            return {'outcome': 'not_init', 'detail': 'Dependency scope changed; review /setup python'}
        code = ('import importlib,importlib.metadata as m,json,sys;'
                'expected=json.loads(sys.argv[1]);'
                'assert sys.version_info >= (3,11);'
                'assert all(m.version(k)==v for k,v in expected.items()),"Installed package versions changed";'
                '[importlib.import_module(n) for n in json.loads(sys.argv[2])]')
        run([executable, '-I', '-c', code, json.dumps(current['packages']),
             json.dumps(current.get('imports', []))], timeout=30)
        if scope and current.get('skills'):
            run([executable, '-I', '-c', skill_probe(current['skills'])], timeout=30)
        if native_memory:
            memory_root = Path(memory_root) if memory_root else Path(root) / 'native-memory'
            assets = json.loads((memory_root / 'embedding.json').read_text(encoding='utf-8'))
            for asset in assets['files']:
                path = Path(asset['path'])
                if not path.is_file() or path.stat().st_size != asset['bytes']:
                    raise RuntimeError('Prepared memory assets are missing or incomplete')
                with path.open('rb') as stream:
                    if hashlib.file_digest(stream, 'sha256').hexdigest() != asset['sha256']:
                        raise RuntimeError('Prepared memory asset hash mismatch')
            if not current.get('memory_ready'):
                return {'outcome': 'not_init', 'detail': 'Memory functional validation is required'}
        return {'outcome': 'ready', 'detail': 'Local interpreter and dependencies verified'}
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.TimeoutExpired) as exc:
        return {'outcome': 'error', 'detail': str(exc)}


def prepare(root, requirements, *, python=sys.executable, skills=(), offline=False, native_memory=False, memory_root=None, scope='', uv='uv', reuse=True, publish=True, managed_requirements=()):
    root, requirements = Path(root).resolve(), Path(requirements).resolve()
    memory_root = Path(memory_root).resolve() if memory_root else root / 'native-memory'
    temporary_directory = root / 'tmp'
    temporary_directory.mkdir(parents=True, exist_ok=True)
    invoke = lambda command: run(command, temporary=temporary_directory)
    skill_paths = [Path(row['file']).parent.resolve() for row in skills if (Path(row['file']).parent / 'pyproject.toml').is_file()]
    digest = hashlib.sha256(requirements.read_bytes() + str(python).encode() + str(native_memory).encode() + str(memory_root).encode())
    managed_requirements = sorted(set(managed_requirements))
    if managed_requirements:
        digest.update(json.dumps(managed_requirements).encode())
    for path in skill_paths:
        digest.update(str(path).encode())
        digest.update((path / 'pyproject.toml').read_bytes())
    identity = digest.hexdigest()
    native_code = ('import sys;sys.path.insert(0,' + repr(str(Path(__file__).resolve().parents[1])) + ');'
                   'from vivy_workbench.memory import prepare_assets, probe;'
                   'prepare_assets(' + repr(str(memory_root)) + ', offline=' + repr(offline) + ');'
                   'probe(' + repr(str(memory_root)) + ')')
    stage('lock')
    with environment_lock(root):
        published = root / 'current.json'
        if reuse and published.is_file():
            try:
                current = json.loads(published.read_text(encoding='utf-8'))
                if current.get('preparation_key', current['identity']) == identity and current.get('scope', '') == scope and Path(current['executable']).is_file():
                    if check(root, scope=scope, native_memory=native_memory, memory_root=memory_root)['outcome'] != 'ready':
                        raise RuntimeError('Published environment needs repair')
                    invoke([current['executable'], '-I', '-c', 'import sys; assert sys.version_info >= (3,11)'])
                    invoke([uv, 'pip', 'check', '--python', current['executable']])
                    invoke([current['executable'], '-I', '-c', skill_probe(skills)])
                    if native_memory:
                        invoke([current['executable'], '-I', '-c', native_code])
                    return current
            except (ValueError, KeyError, RuntimeError):
                pass  # Explicit prepare can repair a damaged publication/environment.
        started = time.monotonic()
        environment = root / 'environments' / uuid.uuid4().hex
        stage('environment')
        invoke([uv, 'venv', str(environment), '--python', str(python), '--no-python-downloads'] + (['--offline'] if offline else []))
        executable = environment / ('Scripts/python.exe' if os.name == 'nt' else 'bin/python')
        stage('dependencies')
        invoke([uv, 'pip', 'sync', '--python', str(executable), '--require-hashes', str(requirements)] + (['--offline'] if offline else []))
        if skill_paths:
            stage('trusted-skills')
            # uv's constraint option splits whitespace in native paths; a file
            # URI preserves the exact Windows path, including spaces.
            command = [uv, 'pip', 'install', '--python', str(executable), '--constraint', requirements.as_uri()]
            for path in skill_paths:
                command.extend(['--editable', str(path)])
            invoke(command + (['--offline'] if offline else []))
        if managed_requirements:
            stage('managed-dependencies')
            invoke([uv, 'pip', 'install', '--python', str(executable), '--constraint', requirements.as_uri()] + (['--offline'] if offline else []) + ['--'] + managed_requirements)
        versions = invoke([str(executable), '-I', '-c', 'import importlib.metadata,json,sys; assert sys.version_info >= (3,11); print(json.dumps({d.metadata["Name"]:d.version for d in importlib.metadata.distributions()}))'])
        imports = ['mcp'] if 'mcp==' in requirements.read_text(encoding='utf-8') else []
        if native_memory:
            imports += ['mempalace', 'chromadb', 'onnxruntime']
        stage('imports')
        invoke([str(executable), '-I', '-c', 'import importlib;[importlib.import_module(n) for n in ' + repr(imports) + ']'])
        invoke([str(executable), '-I', '-c', skill_probe(skills)])
        if native_memory:
            stage('memory-assets-and-probe')
            invoke([str(executable), '-I', '-c', native_code])
        environment_identity = hashlib.sha256((identity + str(executable) + versions).encode()).hexdigest()
        current = {'identity': environment_identity, 'preparation_key':identity, 'executable': str(executable), 'packages': json.loads(versions),
                   'seconds': time.monotonic() - started, 'scope': scope, 'imports': imports,
                   'skills': list(skills),
                   'managed_requirements': managed_requirements,
                   'memory_ready': native_memory, 'memory_root': str(memory_root) if native_memory else None}
        if not publish:
            stage('staged')
            return current
        stage('publish')
        temporary = root / (uuid.uuid4().hex + '.json.tmp')
        with temporary.open('x', encoding='utf-8') as stream:
            json.dump(current, stream)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, published)
        return current


if __name__ == '__main__':
    print(json.dumps(prepare(sys.argv[1], sys.argv[2], python=sys.executable,
                             skills=json.loads(os.environ.get('VIVY_WORKBENCH_SKILLS', '[]')))))

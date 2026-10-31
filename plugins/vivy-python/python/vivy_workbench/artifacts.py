"""Explicit complete artifacts. A failed write never yields a complete descriptor."""
import json
import os
from pathlib import Path
import uuid


def save(value, media_type='text/plain', *, max_bytes=256 * 1024 * 1024):
    if media_type == 'application/json':
        data = json.dumps(value, ensure_ascii=False).encode('utf-8')
    elif isinstance(value, bytes):
        data = value
    elif isinstance(value, str):
        data = value.encode('utf-8', errors='backslashreplace')
    else:
        raise TypeError('Use application/json for structured values')
    if len(data) > max_bytes:
        raise ValueError(f'Artifact exceeds {max_bytes} bytes; no complete artifact saved')
    root = os.environ.get('VIVY_WORKBENCH_ARTIFACTS')
    if not root:
        raise RuntimeError('Host did not configure an artifact directory')
    from . import repl
    cell = repl._current_cell.get()
    # Hash the protocol ID so arbitrary user text cannot escape the output root.
    import hashlib
    partition = hashlib.sha256((cell or 'background').encode()).hexdigest()[:24]
    directory = Path(root) / partition
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / (uuid.uuid4().hex + '.artifact')
    try:
        with path.open('xb') as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
    except BaseException:
        path.unlink(missing_ok=True)
        raise
    return {'path': str(path.resolve()), 'bytes': len(data), 'media_type': media_type,
            'complete': True, 'request_id': cell}

"""Fixed assertion runner for frozen pure-Python Skills; not an OS sandbox."""
import asyncio
import contextlib
import inspect
import io
import json
from pathlib import Path


class BoundedOutput(io.TextIOBase):
    def __init__(self):
        self.bytes = 0

    def write(self, text):
        self.bytes += len(text.encode('utf-8', errors='replace'))
        if self.bytes > 1024 * 1024:
            raise RuntimeError('validation output exceeds 1 MiB')
        return len(text)


def main(request_file):
    from .capabilities import bootstrap
    request = json.loads(Path(request_file).read_text(encoding='utf-8'))
    rows = []
    sink = BoundedOutput()
    try:
        with contextlib.redirect_stdout(sink), contextlib.redirect_stderr(sink):
            namespace = {}
            records = bootstrap(namespace, request['inventory'])
            for record in records:
                if record.get('revision') and record['state'] != 'ready':
                    raise RuntimeError(record.get('error', 'Managed Skill did not load'))
            selected = next(row for row in records if row['name'] == request['skill']['name'] and row.get('revision') == request['skill']['revision'])
            target = namespace[selected['binding']]
            for case in request['checks']['cases']:
                try:
                    actual = target(*case.get('args', []), **case.get('kwargs', {}))
                    if inspect.isawaitable(actual):
                        actual = asyncio.run(actual)
                    passed = actual == case['expected']
                    rows.append({'passed': bool(passed), 'actual': repr(actual)[:512]})
                except BaseException as exc:
                    rows.append({'passed': False, 'error': f'{type(exc).__name__}: {str(exc)[:512]}'})
        result = {'passed': all(r['passed'] for r in rows), 'assertions': len(rows), 'results': rows}
    except BaseException as exc:
        result = {'passed': False, 'assertions': len(request['checks']['cases']),
                  'error': f'{type(exc).__name__}: {str(exc)[:512]}'}
    print(json.dumps(result, ensure_ascii=True))

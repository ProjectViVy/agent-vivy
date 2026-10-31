"""Real worker protocol regressions; bounded reads make crashes fail promptly."""
import json
import os
import pathlib
import queue
import subprocess
import sys
import threading
import tempfile
import unittest

ROOT = pathlib.Path(__file__).parents[1]


class RuntimeTests(unittest.TestCase):
    def setUp(self):
        self.artifacts = tempfile.TemporaryDirectory()
        self.addCleanup(self.artifacts.cleanup)
        self.p = subprocess.Popen([sys.executable, '-u', str(ROOT / 'src/worker.py')],
                                  stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.PIPE, text=True, encoding='utf-8',
                                  env={**os.environ, 'VIVY_WORKBENCH_ARTIFACTS': self.artifacts.name})
        self.events = queue.Queue()
        def read():
            for line in self.p.stdout:
                self.events.put(json.loads(line))
            self.events.put({'event': 'eof'})
        threading.Thread(target=read, daemon=True).start()
        ready = self.next()
        self.addCleanup(self.cleanup)
        self.assertEqual(ready.get('protocol'), 1, ready)

    def cleanup(self):
        if self.p.poll() is None:
            self.p.stdin.close()
            try:
                self.p.wait(3)
            except subprocess.TimeoutExpired:
                self.p.kill()
                self.p.wait()
        for pipe in (self.p.stdin, self.p.stdout, self.p.stderr):
            pipe.close()

    def next(self):
        event = self.events.get(timeout=5)
        self.assertNotEqual(event['event'], 'eof')
        return event

    def send(self, **request):
        self.p.stdin.write(json.dumps(request) + '\n')
        self.p.stdin.flush()

    def run_cell(self, code, rid='cell'):
        self.send(type='execute', id=rid, code=code)
        events = []
        while True:
            event = self.next()
            events.append(event)
            if event['event'] == 'done' and event.get('id') == rid:
                return events

    def test_persistent_tasks_keep_original_output_identity(self):
        first = self.run_cell("import asyncio\nasync def later():\n await asyncio.sleep(.1)\n print('late-output')\ntask=asyncio.create_task(later())", 'first')
        self.assertEqual(first[-1]['status'], 'ok')
        second = self.run_cell('await task\n42', 'second')
        late = [e for e in second if 'late-output' in e.get('text', '')]
        self.assertTrue(late)
        self.assertTrue(all(e['id'] == 'first' for e in late))
        self.assertTrue(any(e.get('text') == '42' and e['id'] == 'second' for e in second))

    def test_host_reply_bypasses_execution_queue(self):
        self.send(type='execute', id='bridge', code="from vivy_workbench import host_request\nawait host_request('mcp.list_connections', {})")
        request = self.next()
        self.assertEqual(request['event'], 'host_request')
        self.send(type='host_reply', id=request['id'], data={'ok': True, 'result': ['fixture']})
        events = [self.next(), self.next()]
        self.assertIn('fixture', events[0].get('text', ''))
        self.assertEqual(events[-1]['status'], 'ok')

    def test_targeted_interrupt_does_not_poison_next_cell(self):
        self.send(type='execute', id='slow', code="import asyncio\nprint('started')\nawait asyncio.sleep(60)")
        self.assertIn('started', self.next().get('text', ''))
        self.send(type='interrupt', id='slow')
        while self.next()['event'] != 'done':
            pass
        events = self.run_cell('40+2', 'after')
        self.assertEqual(events[-1]['status'], 'ok')
        self.assertTrue(any(e.get('text') == '42' for e in events))

    def test_failure_keeps_variables_and_invalid_request_is_reported(self):
        self.run_cell('answer=42')
        self.assertEqual(self.run_cell("raise ValueError('expected')")[-1]['status'], 'error')
        self.assertTrue(any(e.get('text') == '42' for e in self.run_cell('answer')))
        self.send(type='unknown')
        self.assertEqual(self.next().get('ename'), 'ProtocolError')

    def test_typed_memory_request_never_executes_arguments_or_rebound_objects(self):
        self.run_cell("memory = object(); untouched = 42")
        self.send(type='memory', id='typed', action='get', arguments={'identifier': "__import__('os')._exit(7)"})
        first = self.next()
        self.assertEqual(first.get('id'), 'typed', first)
        self.assertEqual(first['event'], 'error')
        self.assertEqual(self.next()['status'], 'error')
        self.assertTrue(any(e.get('text') == '42' for e in self.run_cell('untouched', 'still-alive')))

    def test_typed_memory_rejects_arbitrary_operation_names(self):
        self.send(type='memory', id='invalid', action='__getattribute__', arguments={'name': 'anything'})
        error = self.next()
        self.assertEqual(error.get('id'), 'invalid', error)
        self.assertEqual(error.get('ename'), 'ValueError', error)
        self.assertEqual(self.next()['status'], 'error')

    def test_repr_spills_before_truncation_and_unknown_mime_is_preserved(self):
        events = self.run_cell("'中文' * 200000")
        artifacts = [e['artifact'] for e in events if e['event'] == 'artifact']
        self.assertTrue(artifacts, events[-1])
        self.assertEqual(pathlib.Path(artifacts[0]['path']).read_text(encoding='utf-8'), repr('中文' * 200000))
        events = self.run_cell("from vivy_workbench.repl import emit\nemit({'application/x-fixture': {'value': 42}})")
        displays = [e for e in events if e['event'] == 'display']
        self.assertEqual(json.loads(pathlib.Path(displays[0]['artifact']['path']).read_text()), {'application/x-fixture': {'value': 42}})

    def test_large_error_preserved_before_truncation(self):
        events = self.run_cell("raise ValueError('complete-error-' * 20000)")
        artifacts = [event['artifact'] for event in events if event['event'] == 'artifact']
        self.assertTrue(artifacts)
        self.assertIn('complete-error-' * 20000, pathlib.Path(artifacts[0]['path']).read_text(encoding='utf-8'))


if __name__ == '__main__':
    unittest.main()

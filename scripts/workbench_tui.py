"""Windows terminal Python status, state, cancellation and reset smoke."""
import json
import os
from pathlib import Path
import sys
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer
from mock_gateway import Handler, text_of
from plan_tui import View, command, skip_initialization
from tui import Terminal


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        last = body['messages'][-1]
        if last['role']=='tool':
            return self.reply(body, text='PYTHON_TURN_DONE ' + text_of(last)[-200:])
        prompt = next(text_of(m) for m in reversed(body['messages']) if m['role']=='user')
        if 'WAIT' in prompt:
            code="import asyncio\nprint('PYTHON_WAITING')\nawait asyncio.sleep(60)"
        elif 'FRESH' in prompt:
            code="assert 'value' not in globals()\nprint('PYTHON_FRESH_CONFIRMED')"
        else:
            code="value=42\nprint('PYTHON_STATE_READY')"
        return self.reply(body, tool='python_repl', call={'code':code})


def main():
    server = ThreadingHTTPServer(('127.0.0.1',0),Gateway)
    threading.Thread(target=server.serve_forever,daemon=True).start()
    terminal=None
    try:
        with tempfile.TemporaryDirectory(prefix='vivy-workbench-tui-', ignore_cleanup_errors=True) as scratch:
            root=Path(scratch)
            home=root/'home'
            home.mkdir()
            (home/'settings.toml').write_text(f'[trust]\ndefault="never"\n[gateway]\nbase_url="http://127.0.0.1:{server.server_port}/v1"\nmodel="fixture"\n',encoding='utf-8')
            env=dict(os.environ,RIG_HOME=str(home),RIG_SECRET_GATEWAY='test-key')
            for key in list(env):
                if key.upper().endswith('_PROXY'): del env[key]
            terminal=Terminal([str(Path(sys.argv[1]).resolve())],root,env)
            view=View(terminal)
            skip_initialization(terminal, view)
            view.wait('terminal ready',lambda t:'idle' in t)
            view.wait('initial footer telemetry',lambda t:'Py ' not in t.splitlines()[-1])
            # Idle clock wakeups must rotate the footer without a keypress or a model call.
            view.wait('idle footer switches to Python',lambda t:'Py unprepared' in t.splitlines()[-1])
            switched = time.monotonic()
            view.wait('idle footer returns to telemetry',lambda t:'Py ' not in t.splitlines()[-1])
            assert 4 <= time.monotonic() - switched <= 7, 'footer rotation should take five seconds'
            terminal.type('DRAFT_KEEP',enter=False)
            view.wait('rotation keeps input draft',lambda t:'DRAFT_KEEP' in t and 'Py unprepared' in t.splitlines()[-1])
            terminal.type(b'\x15',enter=False)  # Ctrl+U clears the draft without submitting.
            command(terminal,view,'/python status')
            view.wait('Python status visible',lambda t:'Python initialization:' in t)
            terminal.type('STATE')
            view.wait('Python output visible',lambda t:'PYTHON_STATE_READY' in t and 'idle' in t.splitlines()[-1])
            view.wait('live worker footer',lambda t:'Py ready' in t.splitlines()[-1] and 'Mem not init' in t.splitlines()[-1])
            terminal.type('WAIT')
            view.wait('Python awaiting cancellation',lambda t:'PYTHON_WAITING' in t and 'Esc stop' in t)
            view.wait('running worker footer',lambda t:'Py running' in t.splitlines()[-1] and 'Esc stop' in t)
            terminal.type(b'\x1b',enter=False)
            view.wait('Python cancellation returns idle',lambda t:'idle' in t.splitlines()[-1] and 'Esc stop' not in t)
            terminal.type('FRESH')
            view.wait('cancelled Python starts fresh',lambda t:'PYTHON_FRESH_CONFIRMED' in t and 'idle' in t.splitlines()[-1])
            command(terminal,view,'/python reset')
            view.wait('reset visible',lambda t:'Python closed' in t)
            command(terminal,view,'/quit')
            terminal.read_for(2)
            assert not terminal.process.isalive()
            # Missing interpreter gives a deterministic preparation failure without network.
            with (home/'settings.toml').open('a',encoding='utf-8') as settings:
                settings.write('\n[python]\nexecutable="C:/nonexistent-vivy-fixture/python.exe"\n')
            terminal=Terminal([str(Path(sys.argv[1]).resolve())],root,env)
            view=View(terminal)
            skip_initialization(terminal, view)
            view.wait('failure fixture ready',lambda t:'idle' in t)
            command(terminal,view,'/python prepare')
            view.wait('preparation failure pins footer',lambda t:'Py error' in t.splitlines()[-1])
            terminal.read_for(5.5)
            assert 'Py error' in view.text().splitlines()[-1]
            command(terminal,view,'/python reset')
            view.wait('worker reset preserves initialization error',lambda t:'Py error' in t.splitlines()[-1])
            command(terminal,view,'/quit')
            terminal.read_for(2)
            assert not terminal.process.isalive()
            print('PASS: Windows Python terminal status, output, cancellation, fresh state, reset and exit')
    finally:
        if terminal and terminal.process.isalive(): terminal.process.terminate(force=True)
        server.shutdown()


if __name__=='__main__':main()

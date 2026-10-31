"""Native Windows initialization, remembered skip and explicit provider probe."""
import json
import os
from pathlib import Path
import sys
import tempfile
import threading
from http.server import ThreadingHTTPServer
from mock_gateway import Handler
from plan_tui import View, command
from tui import Terminal


class Gateway(Handler):
    records = []
    fail = False
    block = threading.Event()
    block.set()
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        self.records.append(body)
        if not self.block.is_set():
            self.block.wait(10)
            return  # Cancellation closes the client; no synthetic response is needed.
        if self.fail:
            self.send_error(401, 'synthetic authentication failure')
        elif body.get('tools'):
            self.reply(body, tool='vivy_setup_ping', call={})
        else:
            self.reply(body, text='OK')


def choose(terminal, index):
    terminal.type(b'\x1b[B' * index + b'\r', enter=False)


def main():
    binary = str(Path(sys.argv[1]).resolve())
    server = ThreadingHTTPServer(('127.0.0.1', 0), Gateway)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    terminal = None
    try:
        with tempfile.TemporaryDirectory(prefix='vivy-init-', ignore_cleanup_errors=True) as directory:
            root = Path(directory)
            home = root/'home'
            home.mkdir()
            (home/'settings.toml').write_text('[trust]\ndefault="never"\n', encoding='utf-8')
            env = dict(os.environ, RIG_HOME=str(home))
            for key in list(env):
                if key.upper().endswith('_PROXY') or key.startswith('RIG_SECRET_'):
                    env.pop(key)
            terminal = Terminal([binary], root, env)
            view = View(terminal)
            for title in ['Set up Model provider?', 'Set up Python workbench?', 'Set up MemPalace (optional)?']:
                view.wait(title, lambda t, title=title: title in t)
                choose(terminal, 1)
            view.wait('setup completion', lambda t: 'Initialization checks complete' in t)
            assert len(list((home/'initialization').rglob('*.json'))) == 3
            command(terminal, view, '/quit')
            terminal.read_for(2)
            terminal = Terminal([binary], root, env)
            view = View(terminal)
            view.wait('remembered skips after restart', lambda t: 'Initialization checks complete' in t)
            assert 'Set up ' not in view.text()
            command(terminal, view, '/setup python')
            view.wait('manual setup remains available', lambda t: 'Initialize Python' in t)
            choose(terminal, 0)
            view.wait('consented Python preparation', lambda t: 'Python environment prepared and validated' in t, timeout=120)
            manifest = root/'.vivy/workbench/current.json'
            prepared = json.loads(manifest.read_text())
            assert prepared['scope'] and not prepared['memory_ready']
            command(terminal, view, '/quit')
            terminal.read_for(2)
            # A missing interpreter is repaired under the exact saved grant.
            Path(prepared['executable']).rename(Path(prepared['executable']).with_suffix('.broken'))
            with (home/'settings.toml').open('a', encoding='utf-8') as stream:
                stream.write(f'\n[providers]\ndefault_model="fixture/model"\n[providers.instances.fixture]\nprotocol="openai"\nbase_url="http://127.0.0.1:{server.server_port}/v1"\ncredential={{source="none"}}\n[[providers.instances.fixture.models]]\nid="model"\ntools=true\n')
            terminal = Terminal([binary], root, env)
            view = View(terminal)
            view.wait('approved automatic repair', lambda t: 'Python environment prepared and validated' in t, timeout=120)
            assert json.loads(manifest.read_text())['executable'] != prepared['executable']
            view.wait('configured startup', lambda t: 'Initialization checks complete' in t)
            assert not Gateway.records, 'Startup must not make provider requests'
            command(terminal, view, '/setup provider')
            view.wait('provider setup', lambda t: 'Provider initialization' in t)
            choose(terminal, 3)
            view.wait('explicit streaming and tool probe', lambda t: 'Provider verified' in t)
            assert len(Gateway.records) == 2
            assert all(len(row['messages']) == 1 for row in Gateway.records)
            Gateway.fail = True
            command(terminal, view, '/setup provider')
            view.wait('provider retry menu', lambda t: 'Provider initialization' in t)
            choose(terminal, 3)
            view.wait('provider failure pins shared footer', lambda t: 'Provider error' in t.splitlines()[-1])
            Gateway.block.clear()
            command(terminal, view, '/setup provider')
            view.wait('cancel fixture menu', lambda t: 'Provider initialization' in t)
            choose(terminal, 3)
            view.wait('owned provider test has cancel control', lambda t: 'Testing provider' in t and 'Cancel test' in t)
            choose(terminal, 0)
            view.wait('cancel retains previous failure', lambda t: 'Provider test cancelled' in t and 'Provider error' in t.splitlines()[-1])
            Gateway.block.set()
            command(terminal, view, '/quit')
            terminal.read_for(2)
            assert not terminal.process.isalive()
            print('PASS: initialization ordering, persisted skips, manual retry, no implicit requests, streaming/tools and provider error')
    finally:
        if terminal and terminal.process.isalive(): terminal.process.terminate(force=True)
        server.shutdown()


if __name__ == '__main__':
    main()

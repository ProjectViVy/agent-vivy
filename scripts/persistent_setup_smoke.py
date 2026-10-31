"""Native D4 first-run setup ordering and chat after real Python preparation.

Usage: python scripts/persistent_setup_smoke.py <persistent-agent.exe>
Uses a synthetic gateway and disposable data. Dependency preparation may use
the existing package cache/network; no native memory assets are installed.
"""
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

from mock_gateway import Handler
from plan_tui import View
from tui import Terminal


class Gateway(Handler):
    requests = 0

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        type(self).requests += 1
        self.reply(body, text='SETUP_READY')


def choose(terminal, index=0):
    terminal.type(b'\x1b[B' * index + b'\r', enter=False)


def main():
    binary = str(Path(sys.argv[1]).resolve())
    server = ThreadingHTTPServer(('127.0.0.1', 0), Gateway)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    terminal = None
    try:
        with tempfile.TemporaryDirectory(prefix='vivy D4 setup ', ignore_cleanup_errors=True) as directory:
            root = Path(directory)
            home = root/'home'
            home.mkdir()
            (home/'settings.toml').write_text(
                '[gateway]\nbase_url="http://127.0.0.1:' + str(server.server_port) + '/v1"\n'
                'model="setup-fixture"\ncontext_window=64000\nmax_output_tokens=4096\n'
                '[trust]\ndefault="never"\n', encoding='utf-8')
            env = dict(os.environ, RIG_HOME=str(home), RIG_SESSION=f'{int(time.time())}-{os.getpid()}',
                       RIG_SECRET_GATEWAY='synthetic-setup')
            for name in ('RIG_RUN', 'RIG_LAUNCHER', 'VIVY_DURABLE_PAUSE', 'VIVY_DURABLE_TRACE'):
                env.pop(name, None)
            terminal = Terminal([binary], root, env)
            view = View(terminal)
            view.wait('Python setup', lambda t: 'Set up Python workbench?' in t)
            choose(terminal)
            view.wait('Python approval', lambda t: 'Initialize Python' in t)
            choose(terminal)
            publication = root/'.vivy/workbench/current.json'
            deadline = time.monotonic() + 120
            while time.monotonic() < deadline:
                terminal.read_for(.1)
                screen = view.text()
                if 'Set up MemPalace (optional)?' in screen:
                    assert publication.is_file(), 'Memory setup appeared before Python publication'
                    raw = publication.read_bytes()
                    manifest = json.loads(raw)
                    decisions = list((home/'initialization').glob('*/python.json'))
                    assert len(decisions) == 1
                    decision = json.loads(decisions[0].read_text())
                    assert decision['environment'] == hashlib.sha256(raw).hexdigest(), 'Memory setup appeared before local validation receipt'
                    assert decision['grant'] == manifest['scope']
                    assert Path(manifest['executable']).is_file()
                    break
            else:
                raise AssertionError('Python setup did not finish\n' + screen)
            print('PASS memory setup follows Python publication and local validation receipt', flush=True)
            choose(terminal, 1)
            view.wait('initialization complete', lambda t: 'Initialization checks complete' in t)
            assert Gateway.requests == 0, 'Setup must not send implicit model requests'
            terminal.type('Hello after setup')
            view.wait('foreground reply after setup', lambda t: 'SETUP_READY' in t)
            terminal.type(b'\x04', enter=False)
            terminal.read_for(2)
            assert not terminal.process.isalive()
            terminal = Terminal([binary], root, env)
            view = View(terminal)
            view.wait('same session restored', lambda t: 'Initialization checks complete' in t)
            terminal.type('Hello after reopening')
            view.wait('foreground reply after reopening', lambda t: t.count('SETUP_READY') >= 2)
            terminal.type(b'\x04', enter=False)
            terminal.read_for(2)
            assert not terminal.process.isalive()
            print('PASS native D4 setup, foreground reply and same-session reopen', flush=True)
    finally:
        if terminal and terminal.process.isalive():
            terminal.process.terminate(force=True)
        server.shutdown()


if __name__ == '__main__':
    main()

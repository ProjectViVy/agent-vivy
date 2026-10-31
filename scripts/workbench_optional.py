"""Recipe exclusion and actual headless Python integration (loopback only)."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
from http.server import ThreadingHTTPServer
from mock_gateway import Handler
from subagents_optional import variant, ROOT


class Gateway(Handler):
    def do_POST(self):
        body=json.loads(self.rfile.read(int(self.headers['content-length'])))
        assert 'python_repl' not in {tool['function']['name'] for tool in body['tools']}
        self.server.checked=True
        self.reply(body,text='NO_PYTHON_ACCEPTED')


def main():
    builder=str(Path(sys.argv[1]).resolve())
    headless=variant(builder,'workbench-headless',['terminal'])
    graph=subprocess.check_output(['cargo','tree','--manifest-path',str(ROOT/'.generated/workbench-headless/Cargo.toml'),'-e','normal'],text=True,encoding='utf-8')
    assert 'rig-tui ' not in graph
    subprocess.run([sys.executable,str(ROOT/'scripts/workbench_smoke.py'),str(headless),'--root',str(ROOT/'.workspace/headless-workbench-acceptance')],check=True)
    without=variant(builder,'without-python',['python','terminal'])
    graph=subprocess.check_output(['cargo','tree','--manifest-path',str(ROOT/'.generated/without-python/Cargo.toml'),'-e','normal'],text=True,encoding='utf-8')
    assert 'vivy-python ' not in graph
    server=ThreadingHTTPServer(('127.0.0.1',0),Gateway)
    server.checked=False
    threading.Thread(target=server.serve_forever,daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix='vivy-no-python-') as directory:
            root=Path(directory)
            (root/'settings.toml').write_text(f'[trust]\ndefault="never"\n[gateway]\nbase_url="http://127.0.0.1:{server.server_port}/v1"\nmodel="fixture"\n',encoding='utf-8')
            env=dict(os.environ,RIG_HOME=str(root),RIG_SECRET_GATEWAY='test-key')
            for key in list(env):
                if key.upper().endswith('_PROXY'):del env[key]
            result=subprocess.run([str(without),'-p','Check available tools'],cwd=root,env=env,capture_output=True,text=True,encoding='utf-8',timeout=30)
            assert result.returncode==0 and 'NO_PYTHON_ACCEPTED' in result.stdout,(result.stdout,result.stderr)
            assert server.checked
        print('PASS: no TUI with actual Python workflow; no Python crate and no Python tool when recipe omits it')
    finally:server.shutdown()


if __name__=='__main__':main()

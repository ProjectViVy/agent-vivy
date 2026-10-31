"""Twenty real Coding workbench/child/MCP lifecycle cycles and explicit preparation.

Windows acceptance; all files, Skills and process IDs are owned synthetic fixtures.
"""
import ctypes
from ctypes import wintypes
import json
import os
from pathlib import Path
import statistics
import sys
import threading
import time
from http.server import ThreadingHTTPServer
from mock_gateway import Handler, text_of
from workbench_smoke import JsonAgent, fixture, REPO


def process_info(pid):
    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.OpenProcess.restype = wintypes.HANDLE
    kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    kernel.GetExitCodeProcess.argtypes = [wintypes.HANDLE, ctypes.POINTER(wintypes.DWORD)]
    handle = kernel.OpenProcess(0x1000 | 0x10, False, pid)
    if not handle:
        return None
    class Counters(ctypes.Structure):
        _fields_ = [('cb', wintypes.DWORD), ('faults', wintypes.DWORD)] + [(name, ctypes.c_size_t) for name in ['peak', 'working', 'paged_peak', 'paged', 'nonpaged_peak', 'nonpaged', 'pagefile', 'pagefile_peak']]
    try:
        code = wintypes.DWORD()
        kernel.GetExitCodeProcess(handle, ctypes.byref(code))
        if code.value != 259:
            return None
        counters = Counters()
        counters.cb = ctypes.sizeof(counters)
        psapi = ctypes.WinDLL('psapi')
        psapi.GetProcessMemoryInfo.argtypes = [wintypes.HANDLE, ctypes.c_void_p, wintypes.DWORD]
        if not psapi.GetProcessMemoryInfo(handle, ctypes.byref(counters), counters.cb):
            raise ctypes.WinError(ctypes.get_last_error())
        return counters.working
    finally:
        kernel.CloseHandle(handle)


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        self.server.records.append(body)
        last = body['messages'][-1]
        if 'Stop here' in text_of(last):
            return self.reply(body, text='Synthetic lifecycle summary. Keep Python value=42 for the next warm check.')
        if last['role'] == 'tool':
            return self.reply(body, text='CYCLE_DONE ' + text_of(last))
        user = next(text_of(m) for m in reversed(body['messages']) if m['role']=='user')
        if 'QUEUE_CLOSE' in user:
            codes = ["from pathlib import Path;import asyncio;Path('queue_started').write_text('started');await asyncio.sleep(30)",
                     "from pathlib import Path;Path('queued_after_close').write_text('must not execute')"]
            calls = [{'id':f'queued_{i}', 'type':'function', 'function':{'name':'python_repl','arguments':json.dumps({'code':code})}} for i,code in enumerate(codes)]
            self.send_response(200)
            self.send_header('content-type','text/event-stream' if body.get('stream') else 'application/json')
            self.end_headers()
            if body.get('stream'):
                for i,call in enumerate(calls):call['index']=i
                event={'id':'queued','object':'chat.completion.chunk','created':0,'model':body['model'],'choices':[{'index':0,'delta':{'role':'assistant','tool_calls':calls},'finish_reason':'tool_calls'}]}
                self.wfile.write(b'data: '+json.dumps(event).encode()+b'\n\ndata: [DONE]\n\n')
            else:
                event={'id':'queued','object':'chat.completion','created':0,'model':body['model'],'choices':[{'index':0,'message':{'role':'assistant','tool_calls':calls},'finish_reason':'tool_calls'}]}
                self.wfile.write(json.dumps(event).encode())
            self.wfile.flush()
            return
        if 'REMOVED' in user:
            code = "assert 'dataset_skill' not in globals()\nassert not any(s['name']=='dataset-fixture' for s in workbench.list_skills())\nprint('REMOVAL_CONFIRMED')"
        elif 'WARM' in user:
            code = "assert value==42\nprint('WARM_CONFIRMED')"
        elif 'CRASH' in user:
            code = 'import os;os._exit(9)'
        elif 'TIMEOUT' in user:
            code = 'import asyncio;await asyncio.sleep(60)'
        else:
            code = "import sys,os,subprocess,json\nassert 'value' not in globals()\nvalue=42\nassert dataset_skill([{'value':42}])==42\nchild=subprocess.Popen([sys.executable,'-c','import time;time.sleep(300)'])\nresult=await mcp.call_tool('fixture','second')\nprint('CYCLE_META:'+json.dumps({'worker':os.getpid(),'child':child.pid,'mcp':result['structuredContent']['pid']}))"
        return self.reply(body, tool='python_repl', call={'code':code, 'timeout_secs':1 if 'TIMEOUT' in user else 30})


def wait_closed(pids):
    start = time.monotonic()
    while time.monotonic()-start < 5:
        if all(process_info(pid) is None for pid in pids):
            return time.monotonic()-start
        time.sleep(.05)
    raise AssertionError(f'Owned processes survived close: {pids}')


def main():
    binary, root = Path(sys.argv[1]).resolve(), Path(sys.argv[2]).resolve()
    root.mkdir(parents=True, exist_ok=True)
    home, skill = fixture(root)
    server = ThreadingHTTPServer(('127.0.0.1',0), Gateway)
    server.records = []
    threading.Thread(target=server.serve_forever, daemon=True).start()
    settings = f'[trust]\ndefault="never"\n[gateway]\nbase_url="http://127.0.0.1:{server.server_port}/v1"\nmodel="fixture"\n'
    (home/'settings.toml').write_text(settings, encoding='utf-8')
    env = dict(os.environ, RIG_HOME=str(home), RIG_SECRET_GATEWAY='test-key')
    for key in list(env):
        if key.upper().endswith('_PROXY'):
            del env[key]
    # Preparation may need the user's configured proxy. Runtime loopback does not.
    preparation_env = dict(os.environ, RIG_HOME=str(home), RIG_SECRET_GATEWAY='test-key')
    agent = JsonAgent(binary, root, preparation_env)
    try:
        agent.send('/python prepare')
        agent.until(lambda e: 'Preparing Python.' in e.get('text',''))
        agent.send('/python close')
        agent.until(lambda e: 'Python closed' in e.get('text',''))
        agent.send('/python prepare')
        agent.until(lambda e: 'Python environment prepared and validated.' in e.get('text',''), timeout=180)
        agent.send('/python prepare')
        agent.until(lambda e: 'Python environment prepared and validated.' in e.get('text',''), timeout=180)
    finally:
        agent.close()
        (root/'prepare-events.json').write_text(json.dumps(agent.events, indent=2), encoding='utf-8')
    manifest = json.loads((root/'.vivy/workbench/current.json').read_text())
    settings += '[python.connections.fixture]\ntype="stdio"\ncommand='+json.dumps(manifest['executable'])+'\nargs='+json.dumps([str(REPO/'plugins/vivy-python/tests/mcp_pages_fixture.py'),'pages'])+'\n'
    (home/'settings.toml').write_text(settings, encoding='utf-8')
    agent = JsonAgent(binary, root, env)
    cold, warm, cleanup, resident, pids = [], [], [], [], []
    try:
        for action in ('close','reset'):
            (root/'queue_started').unlink(missing_ok=True)
            (root/'queued_after_close').unlink(missing_ok=True)
            agent.send('QUEUE_CLOSE')
            deadline=time.monotonic()+15
            while not (root/'queue_started').exists() and time.monotonic()<deadline:time.sleep(.05)
            assert (root/'queue_started').exists(), 'first queued cell did not start'
            agent.send('/python '+action)
            agent.until(lambda e:'Python closed' in e.get('text',''))
            agent.until(lambda e:e['type']=='turn_end')
            assert not (root/'queued_after_close').exists(), 'queued call ran after '+action
        for iteration in range(20):
            start = time.monotonic()
            agent.send(f'CYCLE {iteration}')
            agent.until(lambda e:e['type']=='turn_end')
            cold.append(time.monotonic()-start)
            output = next(e['output'] for e in reversed(agent.events) if e['type']=='tool_result')
            row = json.loads(output.split('CYCLE_META:',1)[1].splitlines()[0])
            pids = list(row.values())
            resident.append(process_info(row['worker']))
            if iteration == 0:
                agent.send('/compact')
                agent.until(lambda e: 'Compacted' in e.get('text',''))
                agent.until(lambda e:e['type']=='turn_end' and e.get('error')=='compacted')
            start = time.monotonic()
            agent.send('WARM')
            agent.until(lambda e:e['type']=='turn_end')
            assert 'WARM_CONFIRMED' in json.dumps(agent.events[-4:])
            warm.append(time.monotonic()-start)
            start = time.monotonic()
            agent.send('/python close' if iteration%2 else '/python reset')
            agent.until(lambda e:'Python closed' in e.get('text',''))
            wait_closed(pids)
            cleanup.append(time.monotonic()-start)
        for mode in ['CRASH','TIMEOUT']:
            agent.send(mode)
            agent.until(lambda e:e['type']=='turn_end')
            assert any(e['type']=='tool_result' and not e['ok'] for e in agent.events[-8:])
            agent.send('CYCLE recovered')
            agent.until(lambda e:e['type']=='turn_end')
            output = next(e['output'] for e in reversed(agent.events) if e['type']=='tool_result')
            pids = list(json.loads(output.split('CYCLE_META:',1)[1].splitlines()[0]).values())
            agent.send('/python reset')
            agent.until(lambda e:'Python closed' in e.get('text',''))
            wait_closed(pids)
        Path(skill['file']).replace(Path(skill['file']).with_suffix('.removed'))
        agent.send('/python reset')
        agent.until(lambda e:'Python closed' in e.get('text',''))
        agent.send('REMOVED')
        agent.until(lambda e:e['type']=='turn_end')
        assert 'REMOVAL_CONFIRMED' in json.dumps(agent.events[-5:])
        system = '\n'.join(text_of(m) for m in server.records[-1]['messages'] if m['role']=='system')
        assert '<name>dataset-fixture</name>' not in system
        print(json.dumps({'cycles':20,'orphans':0,'cold_median_seconds':statistics.median(cold),'warm_median_seconds':statistics.median(warm),'close_max_seconds':max(cleanup),'worker_median_rss_bytes':statistics.median(resident),'prepare_seconds':manifest['seconds']}), flush=True)
    finally:
        agent.close()
        server.shutdown()
        (root/'events.json').write_text(json.dumps(agent.events, indent=2), encoding='utf-8')


if __name__=='__main__':
    main()

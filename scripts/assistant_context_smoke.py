"""D2 real executable checks against a synthetic loopback gateway (Windows).

Usage: assistant_context_smoke.py <vivy.exe> <local-rig> [--memory-assets <prepared-root>]
All sessions/personas are disposable. The optional memory root must contain only
acceptance assets/data. No real-provider or OS/power-loss claim.
"""
import argparse
import base64
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

from durable_probe import probe_environment
from durable_runtime_smoke import wait_paused
from mock_gateway import Handler, text_of
from workbench_smoke import JsonAgent

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / '.workspace/d2'
KEY = 'vivy.assistant-context.v1'


def saved(session, key=KEY):
    result = None
    for path in (session / 'logs').glob('*.jsonl'):
        for line in path.read_bytes().splitlines()[1:]:
            try:
                group = json.loads(line)
                for raw in base64.b64decode(group['data']).splitlines():
                    record = json.loads(raw)
                    if record.get('type') == 'state' and record.get('key') == key:
                        result = record['value']
            except (json.JSONDecodeError, KeyError):
                pass  # writer may be appending the final physical group
    return result


def wait_for(predicate, timeout=45):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = predicate()
        if value:
            return value
        time.sleep(.02)
    raise AssertionError('D2 fixture deadline')


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        self.server.requests.append(body)
        recap = any('Maintain concise event memory' in text_of(m) for m in body['messages'])
        if recap:
            assert not body.get('tools'), 'recap may not invoke tools'
            self.server.recap_started.set()
            if self.server.fail_once:
                self.server.fail_once = False
                return self.reply_error(503, 'synthetic transient failure')
            if self.server.slow:
                self.server.allow_recap.wait(30)
            try:
                return self.reply(body, text='RECAP_COMMITTED')
            except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError):
                return
        last = body['messages'][-1]
        user = next(text_of(m) for m in reversed(body['messages']) if m['role'] == 'user')
        if self.server.reconcile is not None:
            case=self.server.reconcile
            if case['step']==0:
                payload=json.dumps(body)
                assert all(v in payload for v in case['arguments'].values())
                names={t['function']['name'] for t in body.get('tools',[])}
                assert 'memory_get' in names and not names & {'memory_remember','python_repl','shell'}
                case['step']=1
                return self.reply(body,tool='memory_get',call=case['arguments'])
            if case['step']==1:
                assert 'D2 synthetic memory value 42.' in text_of(last)
                case['step']=2
                return self.reply(body,tool='resolve_operation',call={'operation':case['operation'],'generation':0,'evidence':'memory_get returned the complete event with the frozen identity and expected content'})
            return self.reply(body,text='RECONCILED_WITHOUT_REWRITE')
        if 'MEMORY_CASE' in user:
            count = sum(m['role'] == 'tool' for m in body['messages'])
            if count == 0:
                return self.reply(body, tool='memory_remember', call={'content': 'D2 synthetic memory value 42.'})
            if count == 1:
                text = text_of(last)
                record = json.loads(text[text.index('{'):text.rindex('}')+1])
                self.server.memory_record = record
                return self.reply(body, tool='memory_get', call={'identifier': record['id']})
            if count == 2:
                assert 'D2 synthetic memory value 42.' in text_of(last), text_of(last)
                record=json.loads(text_of(last)[text_of(last).index('{'):text_of(last).rindex('}')+1])
                return self.reply(body, tool='memory_search', call={'query': 'synthetic value', 'source_id': record['source_id']})
            assert self.server.memory_record['id'] in text_of(last)
            return self.reply(body, text='MEMORY_DONE')
        if 'TOOL_CASE' in user and last['role'] != 'tool':
            return self.reply(body, tool='shell', call={'command': "printf 'TOOL_CURRENT_CANARY'"})
        if last['role'] == 'tool':
            assert 'TOOL_CURRENT_CANARY' in text_of(last)
        return self.reply(body, text='FOREGROUND_DONE')


def fixture(root, server, env, number=1, memory_assets=None):
    home = root / 'home'
    home.mkdir(exist_ok=True)
    config = (f'[gateway]\nbase_url="http://127.0.0.1:{server.server_port}/v1"\nmodel="d2-fixture"\n'
              'context_window=64000\nmax_output_tokens=4096\n[trust]\ndefault="never"\n')
    if memory_assets:
        publication = json.loads((memory_assets/'current.json').read_text(encoding='utf-8'))
        config += '[python]\nmemory=true\nexecutable=' + json.dumps(publication['executable']) + '\nmemory_root=' + json.dumps(str(memory_assets/'native')) + '\n'
    (home/'settings.toml').write_text(config, encoding='utf-8')
    session = home/'sessions'/f'1793530000-{9500+number}'
    environment = dict(env, RIG_HOME=str(home), RIG_SESSION=session.name, RIG_SECRET_GATEWAY='synthetic-d2',
                       LAPUTA_HOME=str(root/'laputa'))
    return environment, session


def turn(agent, text):
    agent.send(text)
    return agent.until(lambda e: e['type'] == 'turn_end', timeout=60)


def build(builder, rig, env, name, no_python=False):
    source = (ROOT/'recipes/persistent-agent.yaml').read_text(encoding='utf-8')
    source = source.replace('vivy-durable: [autodream]', 'vivy-durable: [wake]')
    source = source.replace('vivy-durable: [wake]', 'vivy-durable: [assistant-context]')
    source = source.replace('name: persistent-agent', f'name: {name}').replace('vivy-durable: [assistant-context]', 'vivy-durable: [assistant-context, checkpoint-probe]')
    if no_python:
        source = re.sub(r'  - name: python\n.*?(?=  - name: persona)', '', source, flags=re.S)
    source = re.sub(r'path: ../plugins/([^\s]+)', lambda m: 'path: ' + json.dumps(str(ROOT/'plugins'/m[1])), source)
    recipe = OUT/f'{name}.yaml'
    recipe.write_text(source, encoding='utf-8')
    began = time.monotonic()
    with (OUT/f'{name}-build.log').open('w', encoding='utf-8') as log:
        subprocess.run([str(builder), 'build', str(recipe), '--rig-path', str(rig)], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
    if no_python:
        graph = subprocess.check_output(['cargo', 'tree', '--offline', '--manifest-path', str(ROOT/'.generated'/f'{name}-rig-dev/Cargo.toml'), '-e', 'normal'], env=env, text=True, encoding='utf-8')
        assert 'vivy-python' not in graph and 'vivy-laputa' in graph
        (OUT/'no-python-tree.txt').write_text(graph, encoding='utf-8')
    return Path(env['CARGO_TARGET_DIR'])/'rig-dev/debug'/f'{name}.exe', round(time.monotonic()-began, 3)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('builder', type=Path)
    parser.add_argument('rig', type=Path)
    parser.add_argument('--memory-assets', type=Path)
    args = parser.parse_args()
    env = probe_environment(os.environ)
    OUT.mkdir(parents=True, exist_ok=True)
    binary, build_seconds = build(args.builder.resolve(), args.rig.resolve(), env, 'd2-matrix')
    lean, lean_seconds = build(args.builder.resolve(), args.rig.resolve(), env, 'd2-no-python', True)
    server = ThreadingHTTPServer(('127.0.0.1', 0), Gateway)
    server.requests = []
    server.slow = server.fail_once = False
    server.reconcile = None
    server.recap_started = threading.Event()
    server.allow_recap = threading.Event()
    threading.Thread(target=server.serve_forever, daemon=True).start()
    report = {'build_seconds': build_seconds, 'no_python_build_seconds': lean_seconds, 'cases': []}
    try:
        with tempfile.TemporaryDirectory(prefix='vivy D2 joint ') as directory:
            root = Path(directory)
            environment, session = fixture(root, server, env)
            agent = JsonAgent(lean, root, environment)
            try:
                (root/'persona.txt').write_text('PERSONA_ONE', encoding='utf-8')
                agent.send('/persona update 0 IDENTITY.MD persona.txt')
                agent.until(lambda e: e['type']=='notice' and 'revision 1' in e.get('text',''))
                turn(agent, 'OLD_RAW_CANARY TOOL_CASE')
                first = server.requests[-2:]
                assert all('PERSONA_ONE' in json.dumps(r) for r in first)
                assert 'TOOL_CURRENT_CANARY' in json.dumps(first[-1])
                assert len(saved(session, 'vivy.durable.v1')['tasks']) == 1
                server.slow = True
                server.recap_started.clear()
                agent.send('/recap refresh')
                assert server.recap_started.wait(15)
                started = time.monotonic()
                turn(agent, 'URGENT_NEW_INPUT')
                latency = time.monotonic()-started
                assert latency < 5, latency
                assert 'OLD_RAW_CANARY' not in json.dumps(server.requests[-1])
                server.slow = False
                server.allow_recap.set()
                wait_for(lambda: (s:=saved(session)) and s['jobs'][0]['phase']=='cancelled')
                assert saved(session)['recap_revision']==0
                server.fail_once=True
                agent.send('/recap refresh')
                wait_for(lambda: (s:=saved(session)) and s['recap_revision']==1)
                (root/'persona.txt').write_text('PERSONA_TWO', encoding='utf-8')
                agent.send('/persona update 1 IDENTITY.MD persona.txt')
                agent.until(lambda e:e['type']=='notice' and 'revision 2' in e.get('text',''))
                turn(agent, 'AFTER_RECAP')
                last=json.dumps(server.requests[-1])
                assert 'RECAP_COMMITTED' in last and 'PERSONA_TWO' in last
                assert 'OLD_RAW_CANARY' not in last and 'PERSONA_ONE' not in last
                assert len(saved(session,'vivy.durable.v1')['tasks'])==3
                assert not any(e.get('type')=='text' and 'RECAP_COMMITTED' in e.get('text','') for e in agent.events)
                report['cases'].append({'case':'projection-preemption-retry-persona','foreground_seconds':round(latency,3), 'log_bytes':sum(p.stat().st_size for p in (session/'logs').glob('*'))})
            finally:
                server.allow_recap.set()
                agent.close()
        for index, stage in enumerate(('after_intent','after_effect','after_result','after_publish')):
            with tempfile.TemporaryDirectory(prefix='vivy D2 crash ') as directory:
                root=Path(directory)
                environment,session=fixture(root,server,env,index+10)
                trace=root/'recap.jsonl'
                environment.update(VIVY_RECAP_PAUSE=stage,VIVY_RECAP_TRACE=str(trace))
                start=len(server.requests)
                agent=JsonAgent(lean,root,environment)
                try:
                    turn(agent,'CRASH_INPUT')
                    agent.send('/recap refresh')
                    wait_paused(agent.process,trace,time.monotonic()+30)
                finally:
                    agent.process.kill()
                    agent.close()
                del environment['VIVY_RECAP_PAUSE']
                began=time.monotonic()
                agent=JsonAgent(lean,root,environment)
                try:
                    wait_for(lambda:(s:=saved(session)) and s['recap_revision']==1)
                    requests=len(server.requests)-start
                    assert requests==(3 if stage=='after_effect' else 2),(stage,requests)
                    assert len(saved(session,'vivy.durable.v1')['tasks'])==1
                    report['cases'].append({'case':stage,'requests':requests,'reopen_seconds':round(time.monotonic()-began,3)})
                finally:
                    agent.close()
        if args.memory_assets:
            with tempfile.TemporaryDirectory(prefix='vivy D2 memory ') as directory:
                root=Path(directory)
                environment,session=fixture(root,server,env,50,args.memory_assets.resolve())
                agent=JsonAgent(binary,root,environment)
                try:
                    turn(agent,'MEMORY_CASE')
                    results=[e for e in agent.events if e.get('type')=='tool_result']
                    assert len(results)==3 and all(e['ok'] for e in results),results
                    operation=next(o for o in saved(session,'vivy.durable.v1')['operations'] if o['kind']=='tool')
                    arguments=operation['request']['function']['arguments']
                    assert arguments['source_id'].startswith('workspace:') and session.name in arguments['event_id']
                    report['cases'].append({'case':'native-memory-typed-write-get-search','requests':4})
                finally:
                    agent.close()
            with tempfile.TemporaryDirectory(prefix='vivy D2 uncertain memory ') as directory:
                root=Path(directory)
                environment,session=fixture(root,server,env,51,args.memory_assets.resolve())
                trace=root/'durable.jsonl'
                environment.update(VIVY_DURABLE_PAUSE='after_effect:tool',VIVY_DURABLE_TRACE=str(trace))
                start=len(server.requests)
                agent=JsonAgent(binary,root,environment)
                try:
                    agent.send('MEMORY_CASE')
                    wait_paused(agent.process,trace,time.monotonic()+60)
                finally:
                    agent.process.kill()
                    agent.close()
                operation=next(o for o in saved(session,'vivy.durable.v1')['operations'] if o['kind']=='tool')
                arguments=operation['request']['function']['arguments']
                server.reconcile={'arguments':{k:arguments[k] for k in ('source_id','event_id')},'operation':operation['id'],'step':0}
                del environment['VIVY_DURABLE_PAUSE']
                agent=JsonAgent(binary,root,environment)
                try:
                    agent.until(lambda e:e['type']=='turn_end',timeout=60)
                    final=wait_for(lambda:(s:=saved(session,'vivy.durable.v1')) and s['tasks'][0]['status']=='completed' and s)
                    writes=[o for o in final['operations'] if o['request'].get('function',{}).get('name')=='memory_remember']
                    assert len(writes)==1 and writes[0]['phase']=='unknown' and writes[0]['resolution']
                    assert server.reconcile['step']==2
                    report['cases'].append({'case':'uncertain-native-write-reconciled-with-get','requests':len(server.requests)-start})
                finally:
                    agent.close()
                    server.reconcile=None
        baseline=OUT/'baseline/persistent-agent.exe'
        if baseline.exists():
            with tempfile.TemporaryDirectory(prefix='vivy D2 incompatible ') as directory:
                root=Path(directory)
                environment,session=fixture(root,server,env,70)
                subprocess.run([str(baseline),'-p','D1_BASELINE'],cwd=root,env=environment,capture_output=True,check=True,timeout=30)
                before={p.name:p.read_bytes() for p in (session/'logs').glob('*')}
                rejected=subprocess.run([str(lean),'-p','D2_MUST_REFUSE'],cwd=root,env=environment,capture_output=True,text=True,encoding='utf-8',timeout=30)
                assert rejected.returncode!=0 and 'saved-state schemas' in rejected.stderr,rejected.stderr
                assert before=={p.name:p.read_bytes() for p in (session/'logs').glob('*')}
                report['cases'].append({'case':'D1-required-state-rejected-with-unchanged-logs'})
        report.update(status='passed',request_bytes=sum(len(json.dumps(r).encode()) for r in server.requests),requests=len(server.requests))
        (OUT/'executable-report.json').write_text(json.dumps(report,indent=2),encoding='utf-8')
        (OUT/'requests.json').write_text(json.dumps(server.requests,indent=2),encoding='utf-8')
        print(json.dumps(report,indent=2))
    finally:
        server.allow_recap.set()
        server.shutdown()
        server.server_close()


if __name__=='__main__':
    main()

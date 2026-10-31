"""D4 native Windows acceptance. Synthetic loopback gateway, disposable sessions.

Usage: autodream_smoke.py <vivy.exe> <local-rig> [--skip-build]
Use --only production --skip-build --binary <path> for a normal executable.
No real-provider, arbitrary generated-code safety or power-loss claim.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

from assistant_context_smoke import fixture, wait_for
from durable_probe import probe_environment
from durable_runtime_smoke import wait_paused
from subagents_smoke import alive
from mock_gateway import Handler, text_of
from workbench_smoke import JsonAgent

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / '.workspace/d4'
KEY = 'vivy.durable.v3'


def states(session):
    latest = {}
    for path in (session/'logs').glob('*.jsonl'):
        for line in path.read_bytes().splitlines()[1:]:
            try:
                group = json.loads(line)
                for raw in base64.b64decode(group['data']).splitlines():
                    record = json.loads(raw)
                    if record.get('type') == 'state' and record.get('key') == KEY:
                        state = record['value']
                        kind = 'dream' if state['evolution']['dream'] else 'main'
                        if state['revision'] >= latest.get(kind, {}).get('revision', -1):
                            latest[kind] = state
            except (ValueError, KeyError):
                pass
    return latest


def publication(root):
    return json.loads((root/'.vivy/workbench/current.json').read_text(encoding='utf-8'))


def publication_id(root):
    return hashlib.sha256((root/'.vivy/workbench/current.json').read_bytes()).hexdigest()


def prepared_fixture(root, server, env):
    environment, session = fixture(root, server, env)
    workbench = root/'.vivy/workbench'
    workbench.mkdir(parents=True)
    value = {'identity':'synthetic-python', 'scope':'synthetic-d4', 'executable':sys.executable,
             'skills':[], 'packages':{}, 'imports':[], 'managed_requirements':[]}
    (workbench/'current.json').write_text(json.dumps(value), encoding='utf-8')
    canonical = str(root.resolve())
    if os.name == 'nt' and not canonical.startswith('\\\\?\\'):
        canonical = '\\\\?\\' + canonical
    local = root/'home/initialization'/hashlib.sha256(canonical.encode()).hexdigest()
    local.mkdir(parents=True)
    (local/'python.json').write_text(json.dumps({'grant':'synthetic-d4', 'environment':publication_id(root)}), encoding='utf-8')
    server.root, server.session = root, session
    server.requests = []
    server.hold = None
    server.release = threading.Event()
    return environment, session


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        self.server.requests.append(body)
        messages = body['messages']
        text = '\n'.join(text_of(m) for m in messages)
        system = '\n'.join(text_of(m) for m in messages if m['role'] == 'system')
        if 'Maintain concise event memory' in text:
            return self.reply(body, text='SYNTHETIC_RECAP')
        previous = [c['function']['name'] for m in messages for c in m.get('tool_calls', [])]
        dream = 'You are AutoDream' in system
        if not dream:
            if 'EVALUATE' in text and 'python_repl' not in previous:
                if self.server.hold:
                    self.server.hold.set()
                    self.server.release.wait(30)
                return self.reply(body, tool='python_repl', call={'code':'print(calculator(3))'})
            return self.reply(body, text='FOREGROUND_OK')
        assert '<persona>' not in system, system
        unknown = re.search(r'Unknown effects: (.*?)\. You choose', system)
        unresolved = [o for o in (json.loads(unknown[1]) if unknown else []) if o['resolution'] is None]
        if unresolved:
            if not previous or previous[-1] != 'dream_status':
                return self.reply(body, tool='dream_status', call={})
            return self.reply(body, tool='resolve_operation', call={'operation':unresolved[0]['operation'],
                'generation':unresolved[0]['generation'], 'evidence':'dream_status inspected committed candidate, validation and adoption status; no blind replay'})
        if 'FEEDBACK_ONLY' in text:
            return self.reply(body, text='No further change supported by these observations.')
        persona = 'PERSONA_UPDATE' in text
        if 'python_repl' not in previous:
            if persona:
                code = "from pathlib import Path\nPath('drafts').mkdir(exist_ok=True)\nPath('drafts/persona.txt').write_text('PERSONA_NATIVE_CANARY',encoding='utf-8')"
            else:
                offset = 1 if 'BOOTSTRAP' in text else 2
                source = f'import time\ndef run(value):\n time.sleep(0.25)\n return value+{offset}\n'
                if 'SLOW_CHECK' in text:
                    source=source.replace('time.sleep(0.25)','time.sleep(2)')
                if 'LONG_CHECK' in text:
                    source = "import time,os,subprocess,sys\nfrom pathlib import Path\ndef run(value):\n Path(" + repr(str(self.server.root/'validation.pid')) + ").write_text(str(os.getpid()))\n p=subprocess.Popen([sys.executable,'-I','-c','import time;time.sleep(300)'])\n Path(" + repr(str(self.server.root/'descendant.pid')) + ").write_text(str(p.pid))\n time.sleep(300)\n return value+2\n"
                code = ("from pathlib import Path\np=Path('drafts/calculator');(p/'src/calculator').mkdir(parents=True,exist_ok=True)\n"
                        "(p/'SKILL.md').write_text('---\\nname: calculator\\ndescription: Native synthetic calculator.\\n---\\nCall calculator(value).\\n',encoding='utf-8')\n"
                        "(p/'pyproject.toml').write_text(\"[project]\\nname='calculator'\\nversion='0.1.0'\\n\",encoding='utf-8')\n"
                        f"(p/'src/calculator/__init__.py').write_text({source!r},encoding='utf-8')")
                if 'DEPENDENCY_CASE' in text:
                    metadata="[project]\nname='calculator'\nversion='0.1.0'\ndependencies=['colorama==0.4.6']\n"
                    code+=f"\n(p/'pyproject.toml').write_text({metadata!r},encoding='utf-8')\n(p/'src/calculator/__init__.py').write_text({'import colorama\n'+source!r},encoding='utf-8')"
            return self.reply(body, tool='python_repl', call={'code':code})
        if 'dream_status' not in previous:
            return self.reply(body, tool='dream_status', call={})
        if 'candidate_submit' not in previous:
            if persona:
                path = self.server.root/'.vivy/laputa/persona.json'
                revision = json.loads(path.read_text())['revision'] if path.exists() else 0
                call = {'target':'persona','source':'drafts/persona.txt','expected_base':str(revision)}
            else:
                expected = 4 if 'BOOTSTRAP' in text else 99 if 'FAIL_CHECK' in text else 5
                call = {'target':'skill','source':'drafts/calculator','name':'calculator',
                        'expected_base':publication_id(self.server.root),
                        'checks':{'cases':[{'args':[3],'expected':expected}],
                                  'require_regression':'REPAIR' in text, 'timeout_secs':60,
                                  'replay_safe':'REPLAY_SAFE' in text}}
            return self.reply(body, tool='candidate_submit', call=call)
        current = states(self.server.session).get('dream', {})
        task = current.get('active') or current['tasks'][-1]['id']
        job = current['evolution']['jobs'][-1] if current.get('evolution',{}).get('jobs') else None
        if not job:
            raise AssertionError('submission did not create a job: '+text)
        if 'validation_wait' not in previous:
            return self.reply(body, tool='validation_wait', call={'job':job['id']})
        return self.reply(body, tool='task_control', call={'action':'finish_silently','task':task,'generation':0})


def build(builder, rig, env):
    source = (ROOT/'recipes/persistent-agent.yaml').read_text(encoding='utf-8')
    source = source.replace('name: persistent-agent','name: d4-matrix').replace('vivy-durable: [autodream]','vivy-durable: [autodream, checkpoint-probe]')
    source = re.sub(r'path: ../plugins/([^\s]+)', lambda m:'path: '+json.dumps(str(ROOT/'plugins'/m[1])), source)
    recipe = OUT/'d4-matrix.yaml'; recipe.write_text(source,encoding='utf-8')
    start = time.monotonic()
    with (OUT/'matrix-build.log').open('w',encoding='utf-8') as log:
        subprocess.run([str(builder),'build',str(recipe),'--rig-path',str(rig)],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
    return time.monotonic()-start


def run_dream(agent, session, reason, job_timings=None):
    initial = states(session)
    before = len(initial.get('main',{}).get('evolution',{}).get('requests',[]))
    known_jobs = {j['id'] for j in initial.get('dream',{}).get('evolution',{}).get('jobs',[])}
    started = {}
    def completed():
        current = states(session)
        now = time.monotonic()
        if job_timings is not None:
            for job in current.get('dream',{}).get('evolution',{}).get('jobs',[]):
                if job['id'] not in known_jobs:
                    started.setdefault(job['id'],now)
                    if job['phase'] in ('published','failed','stale','cancelled','unknown'):
                        job_timings.setdefault(job['id'],round(now-started[job['id']],3))
        main = current.get('main')
        return main and len(main['evolution']['requests'])>before and main['evolution']['requests'][-1]['finished'] and current
    agent.send('/dream run '+reason)
    return wait_for(completed,90)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('builder',type=Path);parser.add_argument('rig',type=Path)
    parser.add_argument('--skip-build',action='store_true')
    parser.add_argument('--binary',type=Path,help='test an already built executable (use --skip-build)')
    parser.add_argument('--only',choices=['live','crash','dependencies','cancel','compat','production'])
    args=parser.parse_args();OUT.mkdir(parents=True,exist_ok=True)
    env=probe_environment(os.environ)
    env.setdefault('CARGO_TARGET_DIR',str(ROOT/'.generated/target'))
    report={'cases':[], 'provider':'synthetic loopback; Windows process-crash only'}
    report['build_seconds']=0 if args.skip_build else round(build(args.builder.resolve(),args.rig.resolve(),env),3)
    binary=args.binary.resolve() if args.binary else Path(env['CARGO_TARGET_DIR'])/'rig-dev/debug/d4-matrix.exe'
    server=ThreadingHTTPServer(('127.0.0.1',0),Gateway)
    threading.Thread(target=server.serve_forever,daemon=True).start()
    try:
        if args.only in (None,'live'):
            with tempfile.TemporaryDirectory(prefix='vivy D4 live ') as directory:
                root=Path(directory);environment,session=prepared_fixture(root,server,env)
                agent=JsonAgent(binary,root,environment)
                start=time.monotonic()
                job_timings={}
                try:
                    result=run_dream(agent,session,'BOOTSTRAP',job_timings)
                    assert result['dream']['evolution']['jobs'][-1]['phase']=='published', result
                    old=publication_id(root)
                    result=run_dream(agent,session,'REPAIR',job_timings)
                    job=result['dream']['evolution']['jobs'][-1]
                    assert job['phase']=='published',job
                    assert job['result']['evidence']['old']['passed'] is False
                    assert job['result']['evidence']['new']['passed'] is True
                    fixed=publication_id(root)
                    assert fixed!=old
                    agent.send('EVALUATE actual repaired callable')
                    wait_for(lambda:(s:=states(session).get('main')) and s['tasks'] and s['tasks'][-1]['status']=='completed')
                    used=[o for o in states(session)['main']['operations'] if o['usage']]
                    assert used[-1]['usage'][0]['revision']==job['skill']['revision'],used
                    assert used[-1]['usage'][0]['result']=='5',used
                    result=run_dream(agent,session,'FEEDBACK_ONLY')
                    evidence=[Path(e['path']).read_text(encoding='utf-8') for e in result['dream']['evolution']['evidence'].values()]
                    assert any(job['skill']['revision'] in e and 'usage' in e for e in evidence)
                    result=run_dream(agent,session,'FAIL_CHECK')
                    assert result['dream']['evolution']['jobs'][-1]['phase']=='failed'
                    assert publication_id(root)==fixed
                    result=run_dream(agent,session,'PERSONA_UPDATE')
                    assert result['dream']['evolution']['jobs'][-1]['phase']=='published'
                    agent.send('PERSONA_FOREGROUND_PROBE')
                    wait_for(lambda:len(states(session)['main']['tasks'])>=2 and states(session)['main']['tasks'][-1]['status']=='completed')
                    assert any('PERSONA_NATIVE_CANARY' in text_of(m) for r in server.requests for m in r['messages'] if m['role']=='system' and 'You are AutoDream' not in text_of(m))
                    assert all('<persona>' not in text_of(m) for r in server.requests for m in r['messages'] if m['role']=='system' and 'You are AutoDream' in text_of(m))
                    # Publication can change while a model generates a tool call.
                    server.hold=threading.Event();server.release.clear()
                    agent.send('EVALUATE pinned while publication changes')
                    assert server.hold.wait(30)
                    agent.send(f'/python rollback {fixed} {old}')
                    rolled=wait_for(lambda:publication_id(root)!=fixed and publication_id(root))
                    server.release.set();server.hold=None
                    wait_for(lambda:len(states(session)['main']['tasks'])>=3 and states(session)['main']['tasks'][-1]['status']=='completed')
                    pinned=[o for o in states(session)['main']['operations'] if o['usage']][-1]
                    assert pinned['publication']==fixed and pinned['usage'][0]['result']=='5',pinned
                    agent.send('EVALUATE new request after rollback')
                    wait_for(lambda:len(states(session)['main']['tasks'])>=4 and states(session)['main']['tasks'][-1]['status']=='completed')
                    current=[o for o in states(session)['main']['operations'] if o['usage']][-1]
                    assert current['publication']==rolled and current['usage'][0]['result']=='4',current
                    report['cases'].append({'case':'generated-repair-validation-adoption-use-feedback-and-persona', 'status':'passed',
                        'seconds':round(time.monotonic()-start,3),'requests':len(server.requests),
                        'job_observed_admission_to_terminal_seconds':job_timings,
                        'workspace_bytes':sum(p.stat().st_size for p in (root/'.vivy').rglob('*') if p.is_file())})
                    report['cases'].append({'case':'publication-during-generation-keeps-planned-interface-and-rollback-adopts-next-call','status':'passed'})
                finally:
                    (OUT/'native-live-state.json').write_text(json.dumps(states(session),indent=2),encoding='utf-8')
                    (OUT/'native-live-requests.json').write_text(json.dumps(server.requests,indent=2),encoding='utf-8')
                    try: agent.close()
                    finally:
                        (OUT/'native-live-stderr.log').write_bytes((root/'agent.stderr.log').read_bytes())
        if args.only in (None,'crash'):
            for stage,safe in [(v,False) for v in ['after_job_admission','after_job_dispatch','after_job_validation','after_adoption_intent','after_activation','after_adoption_receipt','after_job_delivery']]+[('after_job_dispatch',True)]:
                with tempfile.TemporaryDirectory(prefix='vivy D4 crash ') as directory:
                    root=Path(directory);environment,session=prepared_fixture(root,server,env)
                    trace=root/'trace.jsonl'
                    first=JsonAgent(binary,root,dict(environment,VIVY_DURABLE_TRACE=str(trace),VIVY_DURABLE_PAUSE=stage))
                    try:
                        first.send('/dream run BOOTSTRAP SLOW_CHECK'+(' REPLAY_SAFE' if safe else ''))
                        paused=wait_paused(first.process,trace,time.monotonic()+60)
                        before=publication_id(root)
                        first.process.kill();first.process.wait(10)
                    finally:
                        first.close()
                    second=JsonAgent(binary,root,environment)
                    try:
                        result=wait_for(lambda:(s:=states(session).get('main')) and s['evolution']['requests'][0]['finished'] and states(session),90)
                        job=result['dream']['evolution']['jobs'][-1]
                        assert job['phase']==('unknown' if stage=='after_job_dispatch' and not safe else 'published'),job
                        if stage in ['after_activation','after_adoption_receipt','after_job_delivery']:
                            assert publication_id(root)==before,'recovery must reconcile, not republish'
                        if job['phase']=='published':
                            assert job['adoption']['publication']==publication_id(root)
                        assert len(result['dream']['evolution']['jobs'])==1
                        report['cases'].append({'case':'crash-'+stage+('-safe' if safe else ''),'status':'passed','phase':job['phase'],'attempt':job['attempt']})
                    finally:
                        (OUT/('crash-'+stage+('-safe' if safe else '')+'.json')).write_text(json.dumps(states(session),indent=2),encoding='utf-8')
                        second.close()
        if args.only in (None,'dependencies'):
            with tempfile.TemporaryDirectory(prefix='vivy D4 dependencies ') as directory:
                root=Path(directory);environment,session=prepared_fixture(root,server,env)
                agent=JsonAgent(binary,root,environment)
                try:
                    before=publication_id(root)
                    agent.send('/dream run DEPENDENCY_CASE')
                    wait_for(lambda:(s:=states(session).get('dream')) and s['evolution']['jobs'] and s['evolution']['jobs'][-1]['phase']=='waiting_setup',90)
                    assert publication_id(root)==before
                    began=time.monotonic();agent.send('FOREGROUND_WHILE_SETUP_PENDING')
                    wait_for(lambda:(s:=states(session).get('main')) and s['tasks'] and s['tasks'][-1]['status']=='completed')
                    latency=time.monotonic()-began
                    agent.send('/python status')
                    event=agent.until(lambda e:e['type']=='notice' and e.get('text','').startswith('Python initialization:'))
                    report_text=event['text'];scope=json.loads(report_text[report_text.index('{'):report_text.rindex('}')+1])['scope']
                    # Synthetic equivalent of the setup picker decision, only in this
                    # isolated home. The real preparation/validation/publication runs.
                    decision=next((root/'home/initialization').glob('*/python.json'))
                    approved=json.loads(decision.read_text());approved.update(grant=scope,scope=scope,skipped=False)
                    decision.write_text(json.dumps(approved),encoding='utf-8')
                    agent.send('/python prepare')
                    def ready():
                        current=json.loads(decision.read_text())
                        if current.get('last_error'):raise RuntimeError(current['last_error'])
                        s=states(session).get('main')
                        return s and s['evolution']['requests'][0]['finished'] and states(session)
                    result=wait_for(ready,240)
                    job=result['dream']['evolution']['jobs'][-1]
                    assert job['phase']=='published',job
                    assert publication(root)['executable']!=sys.executable
                    assert 'colorama==0.4.6' in publication(root)['managed_requirements']
                    assert job['environment']['manifest']['staged_base']==before
                    report['cases'].append({'case':'new-dependency-pending-setup-stage-validate-publish','status':'passed','foreground_seconds':round(latency,3)})
                finally:
                    (OUT/'dependency-state.json').write_text(json.dumps(states(session),indent=2),encoding='utf-8')
                    agent.close()
        if args.only in (None,'cancel'):
            for mode in ['cancel','exit']:
                with tempfile.TemporaryDirectory(prefix='vivy D4 cancellation ') as directory:
                    root=Path(directory);environment,session=prepared_fixture(root,server,env)
                    agent=JsonAgent(binary,root,environment)
                    closed=False
                    try:
                        before=publication_id(root)
                        agent.send('/dream run LONG_CHECK')
                        wait_for(lambda:(root/'descendant.pid').is_file(),90)
                        parent=int((root/'validation.pid').read_text());child=int((root/'descendant.pid').read_text())
                        assert alive(parent) and alive(child)
                        wait_for(lambda:states(session)['dream']['tasks'][0]['status']=='suspended')
                        agent.send('/dream detach job:1')
                        wait_for(lambda:states(session)['dream']['evolution']['jobs'][0]['skipped'])
                        assert alive(parent) and alive(child),'detaching a wait cancelled its job'
                        began=time.monotonic();agent.send('FOREGROUND_WHILE_VALIDATING')
                        wait_for(lambda:(s:=states(session).get('main')) and s['tasks'] and s['tasks'][-1]['status']=='completed')
                        latency=time.monotonic()-began
                        assert alive(parent),'foreground work cancelled independent validation'
                        if mode=='cancel':
                            agent.send('/dream cancel')
                            wait_for(lambda:states(session)['dream']['evolution']['jobs'][0]['phase']=='cancelled')
                        else:
                            agent.close();closed=True
                        wait_for(lambda:not alive(parent) and not alive(child),15)
                        assert publication_id(root)==before
                        report['cases'].append({'case':'wait-detachment-foreground-priority-and-'+mode+'-process-tree-cleanup','status':'passed','foreground_seconds':round(latency,3)})
                    finally:
                        if not closed:agent.close()
        if args.only in (None,'compat'):
            old_binary=OUT/'baseline/persistent-agent.exe'
            for producer,consumer,label in [(old_binary,binary,'D3-to-D4'),(binary,old_binary,'D4-to-D3')]:
                with tempfile.TemporaryDirectory(prefix='vivy D4 compatibility ') as directory:
                    root=Path(directory);environment,session=prepared_fixture(root,server,env)
                    created=subprocess.run([str(producer),'-p','COMPATIBILITY_SEED'],cwd=root,env=environment,capture_output=True,text=True,encoding='utf-8',timeout=45)
                    assert created.returncode==0,created.stderr
                    def snapshot():
                        return {str(p.relative_to(session)):p.read_bytes() for folder in ['logs','files'] for p in (session/folder).rglob('*') if p.is_file()}
                    before=snapshot();count=len(server.requests)
                    refused=subprocess.run([str(consumer),'-p'],cwd=root,env=environment,capture_output=True,text=True,encoding='utf-8',timeout=30)
                    assert refused.returncode!=0,refused.stdout+refused.stderr
                    assert snapshot()==before and len(server.requests)==count
                    (OUT/(label+'.log')).write_text(refused.stdout+refused.stderr,encoding='utf-8')
                    report['cases'].append({'case':label+'-refusal-without-log-mutation','status':'passed'})
        if args.only=='production':
            with tempfile.TemporaryDirectory(prefix='vivy D4 production ') as directory:
                root=Path(directory);environment,session=prepared_fixture(root,server,env)
                trace=root/'must-not-exist.jsonl'
                agent=JsonAgent(binary,root,dict(environment,VIVY_DURABLE_TRACE=str(trace),VIVY_DURABLE_PAUSE='after_job_dispatch'))
                try:
                    result=run_dream(agent,session,'BOOTSTRAP')
                    assert result['dream']['evolution']['jobs'][-1]['phase']=='published'
                    agent.send('EVALUATE production callable')
                    wait_for(lambda:(s:=states(session).get('main')) and s['tasks'] and s['tasks'][-1]['status']=='completed')
                    use=[o for o in states(session)['main']['operations'] if o['usage']][-1]
                    assert use['usage'][0]['result']=='4'
                    assert not trace.exists(),'diagnostic fault hooks leaked into production build'
                    report['cases'].append({'case':'production-adoption-real-call-no-fault-hooks','status':'passed'})
                finally:agent.close()
        report['status']='passed'
    except Exception as exc:
        report.update(status='failed',error=repr(exc));raise
    finally:
        server.shutdown();server.server_close()
        (OUT/('native-'+(args.only or 'report')+'.json')).write_text(json.dumps(report,indent=2),encoding='utf-8')
    print(json.dumps(report,indent=2))


if __name__=='__main__':
    main()

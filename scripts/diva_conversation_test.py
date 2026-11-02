"""Native durable desktop contract checks using only temporary synthetic data."""
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from http.server import ThreadingHTTPServer
from mock_gateway import Handler, text_of

BINARY = Path(sys.argv.pop(1)).resolve()

class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        self.server.requests.append(body)
        if any('SLOW_CASE' in text_of(m) for m in body['messages']):
            self.server.slow.set()
            self.server.release.wait(15)
        try:
            if any('READ_FIXTURE' in text_of(m) for m in body['messages']):
                if body['messages'][-1]['role']=='tool':self.reply(body,text='READ_TOOL_FINISHED')
                else:self.reply(body,tool='read',call={'path':'fixture.txt'})
            elif any(t.get('function',{}).get('name')=='vivy_setup_ping' for t in body.get('tools',[])):
                self.reply(body, tool='vivy_setup_ping',call={})
            else:
                self.reply(body, text='DIVA_COMMITTED_REPLY')
        except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError):
            pass

class Client:
    def __init__(self, root, session=None, credential=None):
        env = {k:v for k,v in os.environ.items() if not k.startswith(('RIG_', 'VIVY_'))}
        env['RIG_HOME'] = str(root/'home')
        env['RIG_SECRET_GATEWAY'] = credential or 'synthetic-diva'
        if session: env['RIG_SESSION'] = session
        self.p = subprocess.Popen([str(BINARY), '--diva'], cwd=root, env=env,
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.events = queue.Queue()
        self.log = []
        self.errors = []
        def read():
            for line in self.p.stdout: self.events.put(json.loads(line))
            self.events.put(None)
        self.threads = [threading.Thread(target=read),
            threading.Thread(target=lambda:self.errors.append(self.p.stderr.read()))]
        for t in self.threads: t.start()
        self.ready = self.until(lambda e:e['type']=='ready')
        self.number = 0
    def until(self, predicate, timeout=25):
        end=time.monotonic()+timeout
        while time.monotonic()<end:
            event=self.events.get(timeout=max(.01,end-time.monotonic()))
            assert event is not None, self.errors
            self.log.append(event)
            if predicate(event): return event
        raise AssertionError('desktop contract deadline')
    def request(self, method, params=None):
        self.number+=1
        id=f'r{self.number}'
        value={'v':1,'id':id,'method':method}
        if params is not None: value['params']=params
        self.p.stdin.write(json.dumps(value).encode()+b'\n'); self.p.stdin.flush()
        return self.until(lambda e:e['id']==id)
    def close(self):
        if self.p.poll() is None: self.p.kill()
        self.p.wait(timeout=10)
        for t in self.threads: t.join(timeout=10)
        for p in (self.p.stdin,self.p.stdout,self.p.stderr): p.close()

class ConversationTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(prefix='diva-conversation-')
        self.root=Path(self.temp.name)
        self.server=ThreadingHTTPServer(('127.0.0.1',0),Gateway)
        self.server.requests=[]
        self.server.slow=threading.Event(); self.server.release=threading.Event()
        threading.Thread(target=self.server.serve_forever,daemon=True).start()
        (self.root/'home').mkdir()
        (self.root/'home/settings.toml').write_text(
            f'[gateway]\nbase_url="http://127.0.0.1:{self.server.server_port}/v1"\n'
            'model="EXACT/model-id"\ncontext_window=64000\nmax_output_tokens=4096\n'
            '[trust]\ndefault="never"\n',encoding='utf-8')
        self.clients=[]
    def tearDown(self):
        self.server.release.set()
        for c in self.clients: c.close()
        self.server.shutdown(); self.server.server_close(); self.temp.cleanup()
    def start(self,session=None):
        c=Client(self.root,session); self.clients.append(c); return c
    def state_until(self,c,predicate):
        end=time.monotonic()+25
        while time.monotonic()<end:
            state=c.request('snapshot')['data']
            if predicate(state): return state
            time.sleep(.03)
        self.fail(str(c.log[-5:]))
    def test_admission_reopen_history_and_conflict(self):
        c=self.start(); session=c.ready['data']['session']
        params={'request_id':'stable-1','text':'Hello 中文\nsecond line'}
        submitted=c.request('submit',params)
        self.assertEqual(submitted['type'],'reply',submitted)
        state=self.state_until(c,lambda s: any(t['status']=='completed' for t in s['tasks']))
        task=state['tasks'][0]
        duplicate=c.request('submit',params)
        self.assertEqual(duplicate['data']['task'],task['id'])
        self.assertEqual(duplicate['data']['phase'],'committed')
        conflict=c.request('submit',dict(params,text='different'))
        self.assertEqual(conflict['data']['code'],'conflict')
        first=c.request('history',{'cursor':None,'limit':1})['data']
        self.assertEqual(first['messages'][0]['role'],'user')
        second=c.request('history',{'cursor':first['next_cursor'],'limit':1})['data']
        self.assertEqual(second['messages'][0]['text'],'DIVA_COMMITTED_REPLY')
        self.assertNotEqual(first['messages'][0]['id'],second['messages'][0]['id'])
        c.request('shutdown'); self.assertEqual(c.p.wait(timeout=10),0)
        restored=self.start(session)
        lookup=restored.request('admission',params)
        self.assertEqual(lookup['data']['phase'],'committed')
        history=restored.request('history',{'cursor':None,'limit':20})['data']
        self.assertEqual([m['id'] for m in history['messages']],
            [first['messages'][0]['id'],second['messages'][0]['id']])
        self.assertEqual(len(restored.request('snapshot')['data']['tasks']),1)
        self.assertEqual(self.server.requests[0]['model'],'EXACT/model-id')
    def test_cancel_generation_and_stale_question(self):
        c=self.start()
        result=c.request('submit',{'request_id':'cancel-1','text':'SLOW_CASE'})
        self.assertEqual(result['type'],'reply',result)
        self.assertTrue(self.server.slow.wait(15))
        state=self.state_until(c,lambda s:bool(s['tasks']))
        task=state['tasks'][0]
        bad=c.request('cancel',{'task':task['id'],'generation':99})
        self.assertEqual(bad['data']['code'],'stale_identity')
        good=c.request('cancel',{'task':task['id'],'generation':task['generation']})
        self.assertEqual(good['type'],'reply',good)
        state=self.state_until(c,lambda s:s['tasks'][0]['status']=='cancelled')
        self.assertEqual(state['tasks'][0]['generation'],task['generation']+1)
        answer=c.request('answer',{'instance':'old','question':'0','choices':[],'text':'yes'})
        self.assertEqual(answer['data']['code'],'stale_identity')

    def test_real_tool_execution_and_committed_projection(self):
        (self.root/'fixture.txt').write_text('DIVA_LOCAL_TOOL_EVIDENCE',encoding='utf-8')
        c=self.start();c.request('submit',{'request_id':'read-tool','text':'READ_FIXTURE'})
        state=self.state_until(c,lambda s:any(t['status']=='completed' for t in s['tasks']))
        tools=state['tools'];self.assertEqual(len(tools),1)
        self.assertEqual(tools[0]['name'],'read');self.assertIn('DIVA_LOCAL_TOOL_EVIDENCE',tools[0]['output'])
        self.assertEqual(c.request('history',{'cursor':None,'limit':100})['data']['messages'][-1]['text'],'READ_TOOL_FINISHED')

    def test_live_question_answer_and_rejection_after_settlement(self):
        path=self.root/'home/settings.toml';path.write_text(path.read_text().replace('default="never"','default="ask"'))
        (self.root/'.rig').mkdir();(self.root/'.rig/ASSISTANT.md').write_text('Synthetic project instructions',encoding='utf-8')
        c=self.start();state=self.state_until(c,lambda s:bool(s['questions']))
        question=state['questions'][0]
        params={'instance':c.ready['instance'],'question':question['id'],'choices':['no'],'text':None}
        bad=c.request('answer',dict(params,choices=['invented']))
        self.assertEqual(bad['data']['code'],'invalid_request')
        answered=c.request('answer',params);self.assertEqual(answered['data']['answered'],True)
        self.state_until(c,lambda s:not s['questions'])
        self.assertEqual(c.request('answer',params)['data']['code'],'stale_identity')

    def test_explicit_provider_checks_are_independent_of_conversation(self):
        c=self.start()
        view=c.request('providers_read')['data']
        accepted=c.request('provider_test',{'generation':view['generation'],'reference':'gateway/EXACT/model-id'})
        self.assertEqual(accepted['type'],'reply',accepted)
        operation=accepted['data']['operation']
        result=next((e for e in c.log if e['type']=='management_completed' and e['data']['operation']==operation),None)
        if result is None:
            result=c.until(lambda e:e['type']=='management_completed' and e['data']['operation']==operation)
        self.assertIsNone(result['data']['error'],result)
        checks=result['data']['result']['data']['checks']
        self.assertEqual({v['kind'] for v in checks},{'text','stream','tools'})
        self.assertTrue(all(v['passed'] for v in checks),checks)
        self.assertEqual(c.request('snapshot')['data']['tasks'],[])
        self.assertEqual(len(self.server.requests),3)
        self.assertTrue(all(r['model']=='EXACT/model-id' for r in self.server.requests))

    def test_large_history_uses_byte_bounded_pages_without_losing_message_ids(self):
        c=self.start()
        for index in range(9):
            accepted=c.request('submit',{'request_id':f'large-{index}','text':'x'*(500*1024)})
            self.assertEqual(accepted['type'],'reply',accepted)
            state=self.state_until(c,lambda s:len(s['tasks'])==index+1)
            task=next(t for t in state['tasks'] if t['request_id']==f'large-{index}')
            c.request('cancel',{'task':task['id'],'generation':task['generation']})
            self.state_until(c,lambda s:all(t['status']=='cancelled' for t in s['tasks']))
        cursor=None;ids=[];pages=0;truncated=0
        while True:
            frame=c.request('history',{'cursor':cursor,'limit':100})
            self.assertLess(len(json.dumps(frame).encode()),4*1024*1024)
            data=frame['data'];ids.extend(m['id'] for m in data['messages']);pages+=1
            truncated+=sum(m['truncated'] for m in data['messages'])
            cursor=data['next_cursor']
            if cursor is None:break
        self.assertEqual(len(ids),9);self.assertEqual(len(set(ids)),9)
        self.assertEqual(truncated,9);self.assertGreater(pages,1)

    def test_provider_config_conflicts_and_secret_is_write_only(self):
        c=self.start()
        read=c.request('providers_read')
        self.assertEqual(read['type'],'reply',read)
        view=read['data']
        config=view['config']
        config['default_model']='gateway/EXACT/model-id'
        save=c.request('providers_save',{'generation':view['generation'],'config':config})
        self.assertEqual(save['type'],'reply',save)
        stale=c.request('providers_save',{'generation':view['generation'],'config':config})
        self.assertEqual(stale['data']['code'],'conflict')
        key='secret-test-fixture-123456789'
        write=c.request('provider_key',{'provider':'gateway','key':key,'generation':save['data']['generation']})
        self.assertEqual(write['type'],'reply',write)
        self.assertNotIn(key,json.dumps(c.request('providers_read')))
        with (self.root/'home/settings.toml').open('a') as f: f.write('\n# external edit\n')
        conflict=c.request('providers_save',{'generation':write['data']['generation'],'config':config})
        self.assertEqual(conflict['data']['code'],'conflict')

    def test_killed_submit_reconciles_before_retry_without_duplicate_tasks(self):
        c=self.start(); session=c.ready['data']['session']
        params={'request_id':'crash-1','text':'SLOW_CASE'}
        self.assertEqual(c.request('submit',params)['type'],'reply')
        c.p.kill(); c.p.wait(timeout=10)
        restored=self.start(session)
        lookup=restored.request('admission',params)
        self.assertEqual(lookup['type'],'reply',lookup)
        result=restored.request('submit',params)
        self.assertEqual(result['type'],'reply',result)
        self.server.release.set()
        state=self.state_until(restored,lambda s:len(s['tasks'])==1)
        self.assertEqual(state['tasks'][0]['request_id'],'crash-1')
        self.assertEqual(len(state['tasks']),1)

    def test_mcp_native_worker_control_lists_real_stdio_tools(self):
        c=self.start()
        def op(method,params=None):
            reply=c.request(method,params)
            self.assertEqual(reply['type'],'reply',reply)
            event=c.until(lambda e:e['type']=='management_completed' and e['data']['operation']==reply['data']['operation'],timeout=40)['data']
            self.assertIsNone(event['error'],event)
            return event['result']['data']
        view=op('mcp_read')
        config={'fixture':{'type':'stdio','command':sys.executable,'args':[str(Path(__file__).resolve().parents[1]/'plugins/vivy-python/tests/mcp_fixture.py')]}}
        op('mcp_save',{'revision':view['revision'],'connections':config})
        result=op('mcp_control',{'server':'fixture','action':'inspect'})
        self.assertEqual(result['state'],'connected')
        self.assertTrue(any(t['name']=='dataset' for t in result['tools']),result)
        closed=op('mcp_control',{'server':'fixture','action':'close'})
        self.assertEqual(closed['state'],'closed')

    def test_mcp_config_reference_roundtrip_cas_and_idle_close(self):
        c=self.start()
        def op(method,params=None):
            reply=c.request(method,params)
            self.assertEqual(reply['type'],'reply',reply)
            return c.until(lambda e:e['type']=='management_completed' and e['data']['operation']==reply['data']['operation'])['data']
        before=op('mcp_read')
        self.assertIsNone(before['error'],before)
        config={'fixture':{'type':'http','url':'http://127.0.0.1:9/mcp','enabled':False,'headers':{'Authorization':{'env':'DIVA_MCP_TEST_TOKEN'}},'env':{},'enabledTools':[],'disabledTools':[],'startupTimeoutMs':3000,'callTimeoutMs':6000}}
        params={'revision':before['result']['data']['revision'],'connections':config}
        saved=op('mcp_save',params)
        self.assertIsNone(saved['error'],saved)
        self.assertEqual(saved['result']['data']['connections'],config)
        self.assertIsNotNone(op('mcp_save',params)['error'])
        closed=op('mcp_control',{'server':'fixture','action':'close'})
        self.assertIsNone(closed['error'],closed)
        self.assertEqual(closed['result']['data']['state'],'not_running')
        report=c.request('python_read')
        self.assertEqual(report['type'],'reply',report)
        self.assertIn('outcome',report['data'])

    def test_skill_import_edit_conflict_and_recoverable_removal(self):
        c=self.start()
        source=self.root/'import-source';source.mkdir()
        original='---\nname: diva-test\ndescription: Fixture\n---\noriginal'
        (source/'SKILL.md').write_text(original)
        def op(method,params=None,fail=False):
            reply=c.request(method,params)
            self.assertEqual(reply['type'],'reply',reply)
            event=c.until(lambda e:e['type']=='management_completed' and e['data']['operation']==reply['data']['operation'])['data']
            if fail:self.assertIsNotNone(event['error'],event)
            else:self.assertIsNone(event['error'],event)
            return event.get('result')
        result=op('skill_import',{'name':'diva-test','source':str(source)})
        skill=next(s for s in result['data']['skills'] if s['name']=='diva-test')
        self.assertTrue(skill['editable'])
        op('skill_import',{'name':'diva-test','source':str(source)},True)
        changed=original.replace('original','edited')
        result=op('skill_save',{'name':skill['name'],'revision':skill['revision'],'text':changed})
        updated=next(s for s in result['data']['skills'] if s['name']=='diva-test')
        op('skill_remove',{'name':'../escape','revision':updated['revision']},True)
        op('skill_save',{'name':skill['name'],'revision':skill['revision'],'text':original},True)
        op('skill_remove',{'name':skill['name'],'revision':updated['revision']})
        self.assertEqual((source/'SKILL.md').read_text(),original)
        self.assertFalse((self.root/'home/skills/diva-test').exists())
        self.assertTrue(list((self.root/'home/skill-recovery').glob('removed-*/SKILL.md')))

    def test_persona_and_prompt_persist_with_conflict_checks(self):
        c=self.start()
        def operation(method,params=None):
            reply=c.request(method,params)
            self.assertEqual(reply['type'],'reply',reply)
            op=reply['data']['operation']
            event=c.until(lambda e:e['type']=='management_completed' and e['data']['operation']==op)
            self.assertIsNone(event['data']['error'],event)
            return event['data']['result']
        value=operation('persona_read')['data']['current']
        updated=operation('persona_update',{'revision':value['revision'],'expected_update_id':value['update_id'],'update_id':'desktop-edit-1','text':'DIVA_PERSONA'})
        self.assertEqual(updated['data']['current']['text'],'DIVA_PERSONA')
        prompts=operation('prompts_read')['data']['prompts']
        prompt=next(p for p in prompts if p['name']=='ASSISTANT.md')
        operation('prompt_save',{'name':prompt['name'],'revision':prompt['revision'],'text':'DIVA_PROMPT'})
        self.assertEqual((self.root/'home/ASSISTANT.md').read_text(),'DIVA_PROMPT')
        value=operation('persona_read')['data']
        self.assertEqual(value['current']['revision'],1)
        self.assertEqual(len(value['history']),1)

if __name__=='__main__': unittest.main()

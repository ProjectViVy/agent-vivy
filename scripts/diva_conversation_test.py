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

if __name__=='__main__': unittest.main()

"""Native persistent prompt editing, retry freezing and role isolation.

Usage: python scripts/prompt_files_smoke.py <persistent-agent.exe>
Synthetic loopback gateway and disposable files only.
"""
import json
import os
from pathlib import Path
import queue
import sys
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

from assistant_context_smoke import fixture, saved, wait_for
from durable_probe import probe_environment
from mock_gateway import Handler, text_of
from workbench_smoke import JsonAgent


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        self.server.requests.append(body)
        system = '\n'.join(text_of(m) for m in body['messages'] if m['role'] == 'system')
        kind = 'recap' if 'RECAP-ONE' in system else 'dream' if 'DREAM-ONE' in system else 'main'
        self.server.kinds.append(kind)
        if self.server.fail == kind:
            self.server.fail = None
            # The retry must survive an invalid edit and reuse its frozen text.
            if kind == 'main':
                source = self.server.root/'laputa/personas/main/IDENTITY.MD'
                source.write_text('IDENTITY-AFTER', encoding='utf-8')
            else:
                name = {'recap': 'RECAP.md', 'dream': 'AUTODREAM.md'}[kind]
                (self.server.home/name).write_bytes(b'\xff')
            return self.reply_error(503, 'synthetic prompt retry')
        return self.reply(body, text={'main': 'MAIN_OK', 'recap': 'RECAP_OK', 'dream': 'DREAM_OK'}[kind])


def system(request):
    return '\n'.join(text_of(m) for m in request['messages'] if m['role'] == 'system')


def turn(agent, text):
    agent.send(text)
    errors = []
    deadline = time.monotonic() + 35
    while time.monotonic() < deadline:
        try:
            event = agent.output.get(timeout=.1)
        except queue.Empty:
            continue
        agent.events.append(event)
        if event['type'] == 'notice' and event.get('level') == 'error':
            errors.append(event.get('text', ''))
        if event['type'] == 'eof':
            raise AssertionError('unexpected agent exit')
        if event['type'] == 'turn_end':
            return event, errors
    raise AssertionError('prompt turn deadline')


def main():
    binary = Path(sys.argv[1]).resolve(strict=True)
    server = ThreadingHTTPServer(('127.0.0.1', 0), Gateway)
    server.requests, server.kinds, server.fail = [], [], None
    threading.Thread(target=server.serve_forever, daemon=True).start()
    agent = None
    try:
        with tempfile.TemporaryDirectory(prefix='vivy prompt files ') as directory:
            root = Path(directory)
            environment, session = fixture(root, server, probe_environment(os.environ))
            home = server.home = root/'home'
            server.root = root
            agent = JsonAgent(binary, root, environment)
            turn(agent, 'hello')
            assert 'a persistent assistant' in system(server.requests[-1])
            assert 'a coding agent' not in system(server.requests[-1])

            (root/'IDENTITY-EDIT.md').write_text('PERSONA-MARKDOWN', encoding='utf-8')
            agent.send('/persona update 0 IDENTITY.MD IDENTITY-EDIT.md')
            agent.until(lambda e: e['type'] == 'notice' and 'revision 1' in e.get('text', ''))
            persona = root/'laputa/personas/main'
            (persona/'MISSION.MD').write_text('MISSION-EDIT', encoding='utf-8')
            (root/'.rig').mkdir()
            (root/'.rig/IDENTITY.MD').write_text('UNTRUSTED-PROJECT', encoding='utf-8')
            turn(agent, 'first edit')
            text = system(server.requests[-1])
            assert text.index('MISSION-EDIT') < text.index('PERSONA-MARKDOWN')
            assert 'UNTRUSTED-PROJECT' not in text

            (persona/'IDENTITY.MD').write_text('IDENTITY-TWO', encoding='utf-8')
            start = len(server.requests)
            server.fail = 'main'
            turn(agent, 'retry this request')
            requests = server.requests[start:]
            assert len(requests) == 2, len(requests)
            assert requests[0]['messages'] == requests[1]['messages']
            assert 'IDENTITY-TWO' in system(requests[1])
            assert 'IDENTITY-AFTER' not in system(requests[1])
            print('PASS persona files, immediate edits, trust isolation and frozen retry', flush=True)

            count = len(server.requests)
            (persona/'MISSION.MD').write_text('   ', encoding='utf-8')
            result, errors = turn(agent, 'invalid persona')
            assert len(server.requests) == count, 'invalid persona reached the gateway'
            assert 'blank' in json.dumps([result, errors, agent.events[-10:]]).lower()
            (persona/'MISSION.MD').write_text('MISSION-EDIT', encoding='utf-8')
            (persona/'IDENTITY.MD').write_text('IDENTITY-THREE', encoding='utf-8')
            turn(agent, 'repaired persona')
            assert 'IDENTITY-THREE' in system(server.requests[-1])
            print('PASS a blank MISSION refuses before dispatch and repair recovers', flush=True)

            (home/'RECAP.md').write_text('RECAP-ONE: Return a short summary, without tools.', encoding='utf-8')
            server.fail = 'recap'
            start = len(server.requests)
            agent.send('/recap refresh')
            wait_for(lambda: (s := saved(session)) and s['recap_revision'] == 1)
            recap = [r for r in server.requests[start:] if 'RECAP-ONE' in system(r)]
            assert len(recap) == 2 and recap[0]['messages'] == recap[1]['messages']
            assert not recap[0].get('tools') and 'PERSONA-MARKDOWN' not in system(recap[0])

            (home/'AUTODREAM.md').write_text('DREAM-ONE: Review evidence; finish if no change is justified.', encoding='utf-8')
            server.fail = 'dream'
            start = len(server.requests)
            agent.send('/dream run Inspect only')
            wait_for(lambda: len([k for k in server.kinds[start:] if k == 'dream']) >= 2)
            dream = [r for r in server.requests[start:] if 'DREAM-ONE' in system(r)]
            assert len(dream) == 2 and dream[0]['messages'] == dream[1]['messages']
            assert 'MISSION-EDIT' not in system(dream[0])
            assert 'IDENTITY-THREE' not in system(dream[0])
            print('PASS editable Recap and Dream prompts, isolated roles and frozen retries', flush=True)
            wait_for(lambda: (s := saved(session, 'vivy.durable.v3')) and s['active'] is None)
            agent.close()
            agent = JsonAgent(binary, root, environment)
            turn(agent, 'after reopening')
            assert 'MISSION-EDIT' in system(server.requests[-1])
            assert 'IDENTITY-THREE' in system(server.requests[-1])
            print('PASS same-session reopen uses the current persona files', flush=True)
            agent.close()
            agent = None
    finally:
        if agent:
            agent.close()
        server.shutdown()
        server.server_close()


if __name__ == '__main__':
    main()

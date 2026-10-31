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
            name = {'main': 'ASSISTANT.md', 'recap': 'RECAP.md', 'dream': 'AUTODREAM.md'}[kind]
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
            agent = JsonAgent(binary, root, environment)
            turn(agent, 'hello')
            assert 'a persistent assistant' in system(server.requests[-1])
            assert 'a coding agent' not in system(server.requests[-1])

            (root/'PERSONA.md').write_text('PERSONA-MARKDOWN', encoding='utf-8')
            agent.send('/persona update 0 PERSONA.md')
            agent.until(lambda e: e['type'] == 'notice' and 'revision 1' in e.get('text', ''))
            (home/'ASSISTANT.md').write_text('ASSISTANT-ONE', encoding='utf-8-sig')
            (root/'.rig').mkdir()
            (root/'.rig/ASSISTANT.md').write_text('UNTRUSTED-PROJECT', encoding='utf-8')
            turn(agent, 'first edit')
            text = system(server.requests[-1])
            assert text.index('PERSONA-MARKDOWN') < text.index('ASSISTANT-ONE')
            assert 'UNTRUSTED-PROJECT' not in text

            (home/'ASSISTANT.md').write_text('ASSISTANT-TWO', encoding='utf-8')
            start = len(server.requests)
            server.fail = 'main'
            turn(agent, 'retry this request')
            requests = server.requests[start:]
            assert len(requests) == 2, len(requests)
            assert requests[0]['messages'] == requests[1]['messages']
            assert 'ASSISTANT-TWO' in system(requests[1])
            assert 'ASSISTANT-ONE' not in system(requests[1])
            print('PASS assistant identity, Markdown edits, trust, persona order and frozen retry', flush=True)

            (home/'ASSISTANT.md').write_text('   ', encoding='utf-8')
            count = len(server.requests)
            result, errors = turn(agent, 'invalid prompt')
            assert len(server.requests) == count, 'invalid prompt reached the gateway'
            assert 'empty' in json.dumps([result, errors, agent.events[-10:]]).lower()
            (home/'ASSISTANT.md').write_text('ASSISTANT-THREE', encoding='utf-8')
            turn(agent, 'repaired prompt')
            assert 'ASSISTANT-THREE' in system(server.requests[-1])
            print('PASS invalid prompt refuses before dispatch and a repaired file recovers', flush=True)

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
            assert 'PERSONA-MARKDOWN' not in system(dream[0])
            assert 'ASSISTANT-THREE' not in system(dream[0])
            print('PASS editable Recap and Dream prompts, isolated roles and frozen retries', flush=True)
            wait_for(lambda: (s := saved(session, 'vivy.durable.v3')) and s['active'] is None)
            agent.close()
            agent = JsonAgent(binary, root, environment)
            turn(agent, 'after reopening')
            assert 'ASSISTANT-THREE' in system(server.requests[-1])
            assert 'PERSONA-MARKDOWN' in system(server.requests[-1])
            print('PASS same-session reopen uses the current Markdown and retained persona', flush=True)
            agent.close()
            agent = None
    finally:
        if agent:
            agent.close()
        server.shutdown()
        server.server_close()


if __name__ == '__main__':
    main()

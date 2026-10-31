"""D3 Windows executable acceptance with a synthetic loopback gateway.

Usage: durable_wake_smoke.py <vivy.exe> <local-rig>
Disposable new sessions only. No real-provider or power-loss claim.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

from assistant_context_smoke import fixture, saved, wait_for
from durable_probe import probe_environment
from durable_runtime_smoke import wait_paused
from mock_gateway import Handler, text_of
from workbench_smoke import JsonAgent

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / '.workspace/d3'
KEY = 'vivy.durable.v2'


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        self.server.requests.append(body)
        if self.server.baseline:
            return self.reply(body, text='D2_BASELINE')
        messages = body['messages']
        text = '\n'.join(text_of(m) for m in messages)
        if 'Maintain concise event memory' in text:
            return self.reply(body, text='RECAP_SYNTHETIC')
        assert 'D3_ORIGINAL' in text, text
        system = '\n'.join(text_of(m) for m in messages if m['role'] == 'system')
        unknown = re.search(r'Unknown effects: (.*?)\. You choose', system)
        operations = json.loads(unknown[1]) if unknown else []
        unresolved = [o for o in operations if o['resolution'] is None]
        if unresolved:
            previous = [c['function']['name'] for m in messages for c in m.get('tool_calls', [])]
            if not previous or previous[-1] != 'wake_status':
                return self.reply(body, tool='wake_status', call={})
            return self.reply(body, tool='resolve_operation', call={
                'operation': unresolved[0]['operation'], 'generation': unresolved[0]['generation'],
                'evidence': 'wake_status inspected committed schedules and pending control; decide a fresh wait only if no schedule exists'})
        notices = [text_of(m) for m in messages if 'Scheduled wake wake:task:1' in text_of(m)]
        # Tools in this fixture use one call ID, like the existing mock gateway.
        # Real execution/recovery must retain logical operation identities too.
        if len(notices) < self.server.waits:
            return self.reply(body, tool='task_control', call={
                'action': 'wait', 'task': 'task:1', 'generation': 0,
                'seconds': self.server.seconds, 'reason': 'D3_RECHECK'})
        if self.server.shell and not any('D3_TOOL_DONE' in text_of(m) for m in messages if m['role'] == 'tool'):
            return self.reply(body, tool='shell', call={'command': "printf 'D3_TOOL_DONE'"})
        return self.reply(body, tool='task_control', call={'action': 'finish_silently', 'task': 'task:1', 'generation': 0})


def build(builder, rig, environment):
    source = (ROOT/'recipes/persistent-agent.yaml').read_text(encoding='utf-8')
    source = source.replace('vivy-durable: [autodream]', 'vivy-durable: [wake]')
    source = source.replace('name: persistent-agent', 'name: d3-matrix').replace('vivy-durable: [wake]', 'vivy-durable: [wake, checkpoint-probe]')
    source = re.sub(r'path: ../plugins/([^\s]+)', lambda m: 'path: '+json.dumps(str(ROOT/'plugins'/m[1])), source)
    recipe = OUT/'d3-matrix.yaml'
    recipe.write_text(source, encoding='utf-8')
    began = time.monotonic()
    with (OUT/'matrix-build.log').open('w', encoding='utf-8') as log:
        subprocess.run([str(builder), 'build', str(recipe), '--rig-path', str(rig)], cwd=ROOT, env=environment, stdout=log, stderr=subprocess.STDOUT, check=True)
    return Path(environment['CARGO_TARGET_DIR'])/'rig-dev/debug/d3-matrix.exe', time.monotonic()-began


def state(session):
    return saved(session, KEY)


def completed(session):
    s = state(session)
    return s and s['tasks'][0]['status'] == 'completed' and s['active'] is None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('builder', type=Path)
    parser.add_argument('rig', type=Path)
    args = parser.parse_args()
    env = probe_environment(os.environ)
    OUT.mkdir(parents=True, exist_ok=True)
    binary, build_seconds = build(args.builder.resolve(), args.rig.resolve(), env)
    server = ThreadingHTTPServer(('127.0.0.1', 0), Gateway)
    server.requests = []
    server.seconds = server.waits = 1
    server.shell = False
    server.baseline = False
    threading.Thread(target=server.serve_forever, daemon=True).start()
    report = {'build_seconds': round(build_seconds, 3), 'cases': []}
    try:
        with tempfile.TemporaryDirectory(prefix='vivy D3 live ') as directory:
            root = Path(directory)
            environment, session = fixture(root, server, env)
            server.waits = 2
            server.shell = True
            agent = JsonAgent(binary, root, environment)
            began = time.monotonic()
            try:
                agent.send('D3_ORIGINAL')
                wait_for(lambda: completed(session))
                agent.until(lambda e: e['type'] == 'turn_end')
                final = state(session)
                assert len(final['tasks']) == 1 and len(final['wakes']) == 2
                assert all(w['delivered'] is not None for w in final['wakes'])
                assert len(server.requests) == 4, len(server.requests)
                before = len(server.requests)
                time.sleep(.3)
                assert before == len(server.requests), 'idle model dispatch'
                assert any(o['kind'] == 'tool' and o['request']['function']['name'] == 'shell' and o['outcome'] is not None for o in final['operations'])
                report['cases'].append({'case': 'live-repeated-waits-tool-silent-finish', 'seconds': round(time.monotonic()-began, 3), 'requests': before,
                    'wake_lateness_ms': [w['fired_ms']-w['deadline_ms'] for w in final['wakes']],
                    'log_bytes': sum(p.stat().st_size for p in (session/'logs').glob('*.jsonl'))})
                (OUT/'live-state.json').write_text(json.dumps(final, indent=2), encoding='utf-8')
            finally:
                agent.close()
        server.waits = 1
        server.shell = False
        stages = ['before_wait_control', 'after_wait_control', 'before_schedule', 'after_schedule', 'before_fire', 'after_fire', 'after_admission', 'after_delivery']
        for index, stage in enumerate(stages):
            with tempfile.TemporaryDirectory(prefix='vivy D3 kill ') as directory:
                root = Path(directory)
                environment, session = fixture(root, server, env, 100+index)
                trace = root/'trace.jsonl'
                environment.update(VIVY_DURABLE_TRACE=str(trace), VIVY_DURABLE_PAUSE=stage)
                start = len(server.requests)
                with (OUT/f'{stage}.log').open('w', encoding='utf-8') as log:
                    process = subprocess.Popen([str(binary), '--json'], cwd=root, env=environment, stdin=subprocess.PIPE, stdout=log, stderr=subprocess.STDOUT, text=True, encoding='utf-8')
                    try:
                        process.stdin.write(json.dumps({'v': 1, 'type': 'send', 'text': 'D3_ORIGINAL'})+'\n')
                        process.stdin.flush()
                        wait_paused(process, trace, time.monotonic()+45)
                    finally:
                        if process.poll() is None:
                            process.kill()
                        process.wait(10)
                        process.stdin.close()
                before = state(session)
                (OUT/f'{stage}-before.json').write_text(json.dumps(before, indent=2), encoding='utf-8')
                del environment['VIVY_DURABLE_PAUSE']
                began = time.monotonic()
                agent = JsonAgent(binary, root, environment)
                try:
                    wait_for(lambda: completed(session))
                    final = state(session)
                    assert len(final['tasks']) == 1
                    assert len(final['wakes']) == 1, final['wakes']
                    assert final['wakes'][0]['delivered'] is not None
                    if before['wakes']:
                        assert before['wakes'][0]['deadline_ms'] == final['wakes'][0]['deadline_ms']
                    report['cases'].append({'case': stage, 'reopen_seconds': round(time.monotonic()-began, 3), 'requests': len(server.requests)-start})
                    (OUT/f'{stage}-after.json').write_text(json.dumps(final, indent=2), encoding='utf-8')
                finally:
                    agent.close()
        with tempfile.TemporaryDirectory(prefix='vivy D3 print ') as directory:
            root = Path(directory)
            environment, session = fixture(root, server, env, 200)
            server.seconds = 3600
            start = len(server.requests)
            result = subprocess.run([str(binary), '-p', 'D3_ORIGINAL'], cwd=root, env=environment, capture_output=True, text=True, encoding='utf-8', timeout=30)
            assert result.returncode == 0, result.stderr
            assert 'live host' in result.stderr
            assert state(session)['tasks'][0]['status'] == 'waiting'
            assert len(server.requests)-start == 1
            before = {p.name: p.read_bytes() for p in (session/'logs').glob('*.jsonl')}
            old = OUT/'baseline/persistent-agent.exe'
            result = subprocess.run([str(old), '-p'], cwd=root, env=environment, capture_output=True, text=True, encoding='utf-8', timeout=15)
            assert result.returncode != 0
            assert before == {p.name: p.read_bytes() for p in (session/'logs').glob('*.jsonl')}
            report['cases'].append({'case': 'print-exits-waiting-and-D2-refuses-unchanged', 'status': 'passed'})
            agent = JsonAgent(binary, root, environment)
            try:
                agent.send('/wake list')
                agent.until(lambda e: e['type'] == 'notice' and 'wake:task:1' in e.get('text',''))
                agent.send('/wake reschedule wake:task:1 0 3600')
                wait_for(lambda: len(state(session)['wakes']) == 2)
                agent.send('/wake cancel wake:task:1 1')
                wait_for(lambda: state(session)['wakes'][1]['phase'] == 'cancelled')
                final = state(session)
                assert len(final['tasks']) == 1 and final['tasks'][0]['status'] == 'suspended'
                assert len(server.requests)-start == 1, 'management command became model input'
                report['cases'].append({'case': 'wake-list-reschedule-cancel-management-only', 'status': 'passed'})
            finally:
                agent.close()
        with tempfile.TemporaryDirectory(prefix='vivy D3 rejects D2 ') as directory:
            root = Path(directory)
            environment, session = fixture(root, server, env, 201)
            server.baseline = True
            result = subprocess.run([str(OUT/'baseline/persistent-agent.exe'), '-p', 'D2_ORIGINAL'], cwd=root, env=environment, capture_output=True, text=True, encoding='utf-8', timeout=30)
            assert result.returncode == 0, result.stderr
            before = {p.name: p.read_bytes() for p in (session/'logs').glob('*.jsonl')}
            result = subprocess.run([str(binary), '-p'], cwd=root, env=environment, capture_output=True, text=True, encoding='utf-8', timeout=15)
            assert result.returncode != 0
            assert before == {p.name: p.read_bytes() for p in (session/'logs').glob('*.jsonl')}
            report['cases'].append({'case': 'D3-refuses-D2-unchanged', 'status': 'passed'})
        report['status'] = 'passed'
    finally:
        server.shutdown()
        server.server_close()
        (OUT/'native-report.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()

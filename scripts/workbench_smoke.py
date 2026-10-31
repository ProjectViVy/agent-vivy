"""Coding + trusted Skill + real MCP + optional native MemPalace acceptance.

Uses a scripted gateway unless --real is explicitly supplied. All data is synthetic.
Run with the system Python; preparation uses pinned dependencies and publishes only
after probes. --memory-root must name an isolated, explicitly prepared asset root.
"""
import argparse
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import threading
import time
from http.server import ThreadingHTTPServer

from mock_gateway import Handler, text_of

REPO = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO / 'plugins/vivy-python/python'))
from vivy_workbench.prepare import prepare

CELLS = [
    "assert any(s.get('binding') == 'dataset_skill' and s['state'] == 'ready' for s in workbench.list_skills())\nassert not any(s['name'] == 'untrusted-fixture' for s in workbench.list_skills())\nassert 'synthetic' in workbench.instructions('dataset-fixture')\nassert (await mcp.list_connections())[0]['connectionId'] == 'fixture'\nassert (await mcp.describe_tool('fixture','dataset'))['title'] == 'Synthetic dataset'\nrows=(await mcp.call_tool('fixture','dataset'))['structuredContent']['rows']\nprint('DISCOVERED')",
    "total=dataset_skill(rows)\nassert total == 42\ncontent='Synthetic dataset sum: 42. 完整结果。\\n'*5000\nartifact=workbench.artifact(content)\nfrom pathlib import Path\nassert Path(artifact['path']).read_text(encoding='utf-8') == content\nassert artifact['complete']\nprint('ARTIFACT_COMPLETE', artifact['bytes'])",
    "value=42\nraise ValueError('EXPECTED_ORDINARY_ERROR')",
    "assert value==42\nprint('STATE_SURVIVED')",
]
MEMORY_CELL = "assert workbench.readiness['memory']=='ready'\nrecord=await memory.remember(content, source_id=SOURCE, event_id='dataset', wing='coding', room='acceptance')\nassert (await memory.remember(content, source_id=SOURCE, event_id='dataset', wing='coding', room='acceptance'))['deduplicated']\nassert (await memory.get(record['id']))['content']==content\nassert (await memory.search('数据总和', source_id=SOURCE, wing='coding', room='acceptance'))[0]['id']==record['id']\nawait memory.close()\nprint('MEMORY_COMPLETE')"


def validate_events(events, native_memory):
    results = [event for event in events if event.get('type') == 'tool_result']
    required = ['DISCOVERED', 'ARTIFACT_COMPLETE', 'STATE_SURVIVED']
    if native_memory:
        required.append('MEMORY_COMPLETE')
    for marker in required:
        assert any(event.get('ok') is True and marker in event.get('output','') for event in results), f'Missing successful execution evidence: {marker}'
    assert any(event.get('ok') is False and 'EXPECTED_ORDINARY_ERROR' in event.get('output','') for event in results), 'Expected ordinary-error recovery was not exercised'


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['content-length'])))
        self.server.records.append(body)
        messages = body['messages']
        user = next(text_of(m) for m in reversed(messages) if m['role'] == 'user')
        last = messages[-1]
        if 'FRESH_CHECK' in user:
            if last['role'] == 'tool':
                return self.reply(body, text=text_of(last))
            return self.reply(body, tool='python_repl', call={'code': "assert 'value' not in globals()\nprint('FRESH_CONFIRMED')"})
        count = sum(m['role'] == 'tool' for m in messages)
        if count < len(self.server.cells):
            return self.reply(body, tool='python_repl', call={'code': self.server.cells[count], 'timeout_secs': 180})
        outputs = '\n'.join(text_of(m) for m in messages if m['role'] == 'tool')
        required = ['DISCOVERED', 'ARTIFACT_COMPLETE', 'EXPECTED_ORDINARY_ERROR', 'STATE_SURVIVED']
        if self.server.memory:
            required.append('MEMORY_COMPLETE')
        assert all(marker in outputs for marker in required), outputs
        return self.reply(body, text='WORKBENCH_ACCEPTED')


class JsonAgent:
    def __init__(self, binary, root, environment):
        self.output = queue.Queue()
        self.events = []
        self.errors = (root / 'agent.stderr.log').open('w', encoding='utf-8')
        self.process = subprocess.Popen([str(binary), '--json'], cwd=root, env=environment,
                                        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=self.errors,
                                        text=True, encoding='utf-8')
        def read():
            for line in self.process.stdout:
                self.output.put(json.loads(line))
            self.output.put({'type': 'eof'})
        threading.Thread(target=read, daemon=True).start()

    def send(self, text):
        self.process.stdin.write(json.dumps({'v': 1, 'type': 'send', 'text': text}) + '\n')
        self.process.stdin.flush()

    def until(self, predicate, timeout=180):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            event = self.output.get(timeout=max(.1, deadline-time.monotonic()))
            self.events.append(event)
            if event['type'] == 'notice' and event.get('level') == 'error':
                raise RuntimeError(event.get('text', 'agent notice error'))
            if event['type'] == 'eof':
                raise RuntimeError('agent exited: ' + str(self.events[-5:]))
            if predicate(event):
                return event
        raise TimeoutError('agent event deadline')

    def close(self):
        self.process.stdin.close()
        try:
            self.process.wait(10)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait(5)
            raise
        finally:
            self.errors.close()


def fixture(root):
    home = root / 'home'
    skill = home / 'skills/dataset'
    skill.mkdir(parents=True, exist_ok=True)
    (skill / 'SKILL.md').write_text('---\nname: dataset-fixture\ndescription: Process synthetic datasets.\n---\nUse dataset_skill(rows) to sum synthetic values.\n', encoding='utf-8')
    (skill / 'pyproject.toml').write_text('[build-system]\nrequires=["hatchling==1.27.0"]\nbuild-backend="hatchling.build"\n[project]\nname="dataset-skill"\nversion="0.1.0"\n[tool.hatch.build.targets.wheel]\npackages=["dataset_skill"]\n', encoding='utf-8')
    (skill / 'dataset_skill').mkdir(exist_ok=True)
    (skill / 'dataset_skill/__init__.py').write_text('def run(rows):\n    return sum(row["value"] for row in rows)\n', encoding='utf-8')
    untrusted = root / '.rig/skills/untrusted'
    untrusted.mkdir(parents=True, exist_ok=True)
    (untrusted / 'SKILL.md').write_text('---\nname: untrusted-fixture\ndescription: Must not load before project trust.\n---\n', encoding='utf-8')
    return home, {'name': 'dataset-fixture', 'description': 'Synthetic dataset', 'file': str(skill / 'SKILL.md')}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('binary', type=Path)
    parser.add_argument('--root', required=True, type=Path)
    parser.add_argument('--memory-root', type=Path)
    parser.add_argument('--real', action='store_true')
    args = parser.parse_args()
    root = args.root.resolve()
    root.mkdir(parents=True, exist_ok=True)
    home, skill = fixture(root)
    memory = args.memory_root.resolve() if args.memory_root else None
    manifest = prepare(root / '.vivy/workbench', REPO / 'plugins/vivy-python' / ('requirements-memory.lock' if memory else 'requirements-core.lock'), skills=[skill], native_memory=bool(memory), memory_root=memory)
    server = ThreadingHTTPServer(('127.0.0.1', 0), Gateway)
    server.records, server.memory = [], bool(memory)
    server.cells = list(CELLS)
    source_id = 'coding-acceptance-' + str(time.time_ns())
    if memory:
        server.cells.append('SOURCE=' + repr(source_id) + '\n' + MEMORY_CELL)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    if args.real:
        assert os.environ.get('RIG_SECRET_GATEWAY'), 'Configure RIG_SECRET_GATEWAY for the explicitly authorized endpoint'
        endpoint = os.environ['OPENAI_BASE_URL']
        model = os.environ['VIVY_TEST_MODEL']
    else:
        endpoint, model = f'http://127.0.0.1:{server.server_port}/v1', 'org/workbench-test'
    settings = f'[trust]\ndefault="never"\n[gateway]\nbase_url={json.dumps(endpoint)}\nmodel={json.dumps(model)}\n[python]\n'
    if memory:
        settings += 'memory=true\nmemory_root=' + json.dumps(str(memory)) + '\n'
    settings += '[python.connections.fixture]\ntype="stdio"\ncommand=' + json.dumps(manifest['executable']) + '\nargs=' + json.dumps([str(REPO / 'plugins/vivy-python/tests/mcp_fixture.py')]) + '\n'
    (home / 'settings.toml').write_text(settings, encoding='utf-8')
    environment = dict(os.environ, RIG_HOME=str(home))
    if not args.real:
        environment['RIG_SECRET_GATEWAY'] = 'test-key'
        for key in list(environment):
            if key.upper().endswith('_PROXY'):
                del environment[key]
    agent = JsonAgent(args.binary.resolve(), root, environment)
    started = time.monotonic()
    try:
        if args.real:
            prompt = 'Validate this synthetic Python workbench workflow. Execute these Python cells in order with python_repl; one cell deliberately raises an ordinary exception, continue with the next. Do not use shell. Report any failed assertion honestly.\n' + '\n\n'.join(server.cells)
        else:
            prompt = 'Run the synthetic workbench acceptance workflow.'
        agent.send(prompt)
        agent.until(lambda e: e['type']=='turn_end', timeout=300)
        transcript = json.dumps(agent.events, ensure_ascii=False)
        validate_events(agent.events, bool(memory))
        expected_content = 'Synthetic dataset sum: 42. 完整结果。\n' * 5000
        assert any(path.read_bytes() == expected_content.encode() for path in (root/'.vivy/workbench/artifacts').rglob('*.artifact')), 'Complete artifact was not independently found'
        if memory:
            code = ("import sys,asyncio;sys.path.insert(0,sys.argv[1]);from vivy_workbench.memory import Memory\n"
                    "async def verify():\n m=Memory(sys.argv[2])\n try:\n  result=await m.get(source_id=sys.argv[3],event_id='dataset')\n  assert result['content']==('Synthetic dataset sum: 42. 完整结果。\\n'*5000)\n finally: await m.close()\nasyncio.run(verify())")
            verification_env = {key:value for key,value in environment.items() if not key.upper().startswith('RIG_SECRET_')}
            subprocess.run([manifest['executable'],'-I','-c',code,str(REPO/'plugins/vivy-python/python'),str(memory),source_id],env=verification_env,check=True,timeout=60)
        if not args.real:
            assert 'WORKBENCH_ACCEPTED' in transcript, transcript[-6000:]
            agent.send('/python reset')
            agent.until(lambda e: 'Python closed' in json.dumps(e))
            agent.send('FRESH_CHECK')
            agent.until(lambda e: e['type']=='turn_end')
            assert 'FRESH_CONFIRMED' in json.dumps(agent.events[-15:])
        print(json.dumps({'accepted': True, 'real_gateway': args.real, 'native_memory': bool(memory), 'seconds': round(time.monotonic()-started, 3), 'prepare_seconds': manifest['seconds'], 'root': str(root)}), flush=True)
    finally:
        agent.close()
        server.shutdown()
        (root / 'events.json').write_text(json.dumps(agent.events, ensure_ascii=False, indent=2), encoding='utf-8')
        if not args.real:
            (root / 'gateway.json').write_text(json.dumps(server.records, ensure_ascii=False), encoding='utf-8')


if __name__ == '__main__':
    main()

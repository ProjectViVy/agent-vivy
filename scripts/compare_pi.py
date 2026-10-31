"""Run Pi and vivy against the same scripted gateway and compare what they send.

Usage: python scripts/compare_pi.py <path-to-vivy> [<pi-checkout>]

<pi-checkout> defaults to .workspace/pi (dependencies installed with
`npm ci --ignore-scripts`, model data present). Only loopback is used. Every
request body is kept in .generated/compare/<agent>/<scenario>.jsonl, and a
summary is printed and written to .generated/compare/summary.md. Exits
non-zero naming any scenario that could not run.
"""
import json
import os
import pathlib
import shutil
import signal
import subprocess
import sys
import tempfile
import struct
import threading
import time
import zlib
from http.server import ThreadingHTTPServer

from mock_gateway import Handler, text_of
from smoke import turns
from tui import Terminal

ROOT = pathlib.Path(__file__).resolve().parent.parent
OUT = ROOT / ".generated" / "compare"
def png(size=64):
    """A valid RGB PNG, which Pi decodes to resize before sending."""
    raw = b"".join(b"\x00" + b"".join(bytes((x * 4 % 256, y * 4 % 256, 128)) for x in range(size))
                   for y in range(size))
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    header = struct.pack(">IIBBBBB", size, size, 8, 2, 0, 0, 0)
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", header) + chunk(b"IDAT", zlib.compress(raw))
            + chunk(b"IEND", b""))


PNG = png()
AGENTS_MD = "# Project rules\n\nMARKER-AGENTS-MD: answer briefly.\n"
LONG = " ".join("word%d" % (i % 97) for i in range(5000))
SKILL = "---\nname: demo-skill\ndescription: MARKER-SKILL demo skill for the comparison.\n---\nSay hi.\n"
TEMPLATE = "---\ndescription: Ask for a review\n---\nMARKER-TEMPLATE review ${1:-everything}.\n"


def tool_names(body):
    return {t["function"]["name"]: t["function"].get("parameters", {}) for t in body.get("tools", [])}


def edit_keys(schema):
    item = schema.get("properties", {}).get("edits", {}).get("items", {}).get("properties", {})
    old = "oldText" if "oldText" in item else "old_text"
    return old, "newText" if old == "oldText" else "new_text"


def since_last_user(messages):
    """The messages after the last user message: this turn's tool exchange."""
    users = [i for i, m in enumerate(messages) if m["role"] == "user"]
    return messages[users[-1] + 1:] if users else messages


def is_summary_request(body):
    messages = body.get("messages", [])
    last = text_of(messages[-1]) if messages else ""
    system = text_of(messages[0]) if messages and messages[0]["role"] in ("system", "developer") else ""
    return ("Stop here" in last or "summar" in last.lower()
            or ("summar" in system.lower() and "word0" not in last and not body.get("tools")))


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        record = {"path": self.path, "bearer": self.headers.get("authorization") == "Bearer test-key", "body": body}
        with open(self.server.capture, "a", encoding="utf-8") as log:
            log.write(json.dumps(record) + "\n")
        scenario = self.server.scenario
        tools = tool_names(body)
        shell = "bash" if "bash" in tools else "shell"
        messages = body["messages"]
        step = sum(m["role"] == "tool" for m in since_last_user(messages))
        if scenario == "failure":
            return self.reply_error(401, "scripted rejection")
        if scenario == "cancel":
            if step == 0:
                return self.reply(body, tool=shell, call={"command": "sleep 300"})
            return self.reply(body, text="stopped")
        if scenario == "tools":
            old, new = edit_keys(tools.get("edit", {}))
            plan = [
                ("write", {"path": "notes.txt", "content": "alpha\n"}),
                ("read", {"path": "notes.txt"}),
                ("edit", {"path": "notes.txt", "edits": [{old: "alpha", new: "beta"}]}),
                (shell, {"command": "cat notes.txt"}),
                ("read", {"path": "missing.txt"}),
            ]
            if step < len(plan):
                name, args = plan[step]
                return self.reply(body, tool=name, call=args)
            return self.reply(body, text="done")
        if scenario == "compaction":
            if is_summary_request(body):
                return self.reply(body, text="SUMMARY of the conversation so far.")
            return self.reply(body, text=LONG)
        return self.reply(body, text="ok " + scenario)


class Agent:
    def __init__(self, name, root, port):
        self.name, self.root, self.port = name, root, port
        self.home = root / f"{name}-home"
        self.work = root / f"{name}-work"
        self.home.mkdir()
        self.work.mkdir()

    def fresh_work(self):
        for path in self.work.iterdir():
            if path.is_dir():
                shutil.rmtree(path)
            else:
                path.unlink()
        (self.work / "AGENTS.md").write_text(AGENTS_MD, encoding="utf-8")
        (self.work / "pic.png").write_bytes(PNG)

    def run(self, *args, timeout=120):
        result = subprocess.run(self.argv(*args), cwd=self.work, env=self.env(), input="",
            text=True, encoding="utf-8", capture_output=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr

    def popen(self, *args):
        # Its own process group, interrupted as a whole, as a terminal's Ctrl-C does.
        return subprocess.Popen(self.argv(*args), cwd=self.work, env=self.env(), stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, encoding="utf-8",
            start_new_session=True)


class Pi(Agent):
    def __init__(self, root, port, checkout):
        super().__init__("pi", root, port)
        self.checkout = checkout
        resolver = checkout / "packages/coding-agent/src/experimental/source-resolver.ts"
        self.resolver = resolver.as_uri()
        (self.home / "models.json").write_text(json.dumps({"providers": {"gateway": {
            "baseUrl": f"http://127.0.0.1:{port}/v1", "api": "openai-completions", "apiKey": "test-key",
            "models": [{"id": "org/model-1", "reasoning": True, "input": ["text", "image"],
                        "contextWindow": 64000, "maxTokens": 8000}]}}}), encoding="utf-8")

    def argv(self, *args):
        cli = self.checkout / "packages/coding-agent/src/cli.ts"
        return ["node", "--import", self.resolver, str(cli), "--provider", "gateway", "--model", "org/model-1", *args]

    def env(self):
        return dict(os.environ, PI_CODING_AGENT_DIR=str(self.home), PI_OFFLINE="1",
                    PI_SKIP_VERSION_CHECK="1", PI_TELEMETRY="0")

    config_dir = ".pi"
    trust_args = ["--approve"]

    def continue_args(self):
        return ["-c"]

    def resource_runs(self):
        return [("default trust", []), ("--approve", ["--approve"])]

    def multi_turn(self, texts):
        return self.run("-p", *texts, timeout=300)[0]

    def thinking(self, text):
        return [self.run("--thinking", "high", "-p", text)[0]]

    def image(self, text):
        return self.run("-p", "@pic.png", text)[0]


class Vivy(Agent):
    def __init__(self, root, port, binary):
        super().__init__("vivy", root, port)
        self.binary = binary
        (self.home / "settings.toml").write_text(
            f'[gateway]\nbase_url = "http://127.0.0.1:{port}/v1"\nmodel = "org/model-1"\n'
            'dialect = "openrouter"\nreasoning = ["low", "medium", "high"]\nimage_input = true\n'
            'context_window = 64000\nmax_output_tokens = 8000\n', encoding="utf-8")

    def argv(self, *args):
        return [str(self.binary), *args]

    def env(self):
        return dict(os.environ, RIG_HOME=str(self.home), RIG_SECRET_GATEWAY="test-key")

    def newest_session(self):
        sessions = sorted((self.home / "sessions").iterdir(), key=lambda p: p.stat().st_mtime)
        return sessions[-1].name

    config_dir = ".rig"
    trust_args = ["--approve"]

    def continue_args(self):
        return ["--resume", self.newest_session()]

    def resource_runs(self):
        return [("default trust", []), ("--approve", ["--approve"])]

    def multi_turn(self, texts):
        turns(self.binary, self.work, self.env(), texts)
        return 0

    def thinking(self, text):
        self.run("-p", "warm up")
        session = self.newest_session()
        return [self.run("--resume", session, "-p", "/effort high")[0],
                self.run("--resume", session, "-p", text)[0]]

    def image(self, text):
        return self.run("-p", "@pic.png " + text)[0]


def sleeps():
    """Live `sleep 300` processes, as (pid, stat)."""
    out = subprocess.run(["ps", "-eo", "pid=,stat=,args="], capture_output=True, text=True).stdout
    found = []
    for line in out.splitlines():
        pid, stat, args = line.split(None, 2)
        if args == "sleep 300" and not stat.startswith("Z"):
            found.append(int(pid))
    return found


def scenario_run(agent, scenario):
    """Runs one scenario; returns a dict of observed outcomes."""
    agent.fresh_work()
    seen = {}
    if scenario == "text":
        seen["exit"] = agent.run("-p", "hello")[0]
    elif scenario == "tools":
        seen["exit"] = agent.run("-p", "edit the notes")[0]
        notes = agent.work / "notes.txt"
        seen["notes.txt"] = notes.read_text() if notes.exists() else None
    elif scenario == "continue":
        agent.run("-p", "first message")
        seen["exit"] = agent.run(*agent.continue_args(), "-p", "second message")[0]
    elif scenario == "image":
        seen["exit"] = agent.image("describe the image")
    elif scenario == "thinking":
        seen["exits"] = agent.thinking("think hard")
    elif scenario == "compaction":
        seen["exit"] = agent.multi_turn([f"LONG {i}" for i in range(8)])
    elif scenario == "resources":
        skill = agent.work / ".agents/skills/demo-skill/SKILL.md"
        skill.parent.mkdir(parents=True)
        skill.write_text(SKILL, encoding="utf-8")
        for folder in (".pi/prompts", ".rig/prompts"):
            (agent.work / folder).mkdir(parents=True)
            (agent.work / folder / "review.md").write_text(TEMPLATE, encoding="utf-8")
        seen["runs"] = {label: agent.run(*args, "-p", "/review the API")[0]
                        for label, args in agent.resource_runs()}
    elif scenario == "context":
        markers = {"home": agent.home / "AGENTS.md", "parent": agent.root / "CLAUDE.md",
                   "agents": agent.work / "AGENTS.md", "override": agent.work / "AGENTS.override.md",
                   "system": agent.work / agent.config_dir / "SYSTEM.md",
                   "append": agent.work / agent.config_dir / "APPEND_SYSTEM.md"}
        for name, path in markers.items():
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(f"MARKER-{name.upper()} context file\n", encoding="utf-8")
        try:
            seen["exit"] = agent.run(*agent.trust_args, "--append-system-prompt", "MARKER-FLAG", "-p", "hello")[0]
        finally:
            for path in (markers["home"], markers["parent"]):
                path.unlink()
    elif scenario == "tui":
        seen.update(tui_round_trip(agent))
    elif scenario == "failure":
        code, out, err = agent.run("-p", "hello")
        seen["exit"] = code
        seen["message"] = (err.strip() or out.strip()).splitlines()[-1:] if (err or out) else []
    elif scenario == "cancel":
        assert not sleeps(), "a `sleep 300` is already running"
        process = agent.popen("-p", "run the long command")
        deadline = time.time() + 60
        while not sleeps() and time.time() < deadline and process.poll() is None:
            time.sleep(0.2)
        seen["tool_started"] = bool(sleeps())
        os.killpg(process.pid, signal.SIGINT)
        try:
            seen["exit"] = process.wait(timeout=20)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            seen["exit"] = "did not exit within 20s of SIGINT"
        time.sleep(0.5)
        left = sleeps()
        seen["sleep_left_running"] = len(left)
        for pid in left:  # clean up what the agent left, so the next run starts clean
            os.kill(pid, signal.SIGKILL)
    return seen


def tui_round_trip(agent):
    """Types a prompt into the agent's terminal view on a pseudo-terminal and
    waits for the scripted reply on screen."""
    terminal = Terminal(agent.argv(), agent.work, agent.env())
    try:
        terminal.read_for(4)
        mark = terminal.mark()
        terminal.type("hello tui")
        return {"reply_on_screen": terminal.read_for(20, until="ok tui", since=mark)}
    finally:
        terminal.close()


def records(path):
    if not path.exists():
        return []
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]


def shape(content):
    if isinstance(content, str):
        return "string"
    if isinstance(content, list):
        return "[" + ",".join(part.get("type", "?") for part in content) + "]"
    return type(content).__name__


def options(body):
    return {k: v for k, v in body.items() if k not in ("messages", "tools", "model")}


def summarize(name, agent_records, seen):
    lines = []
    reqs = [r["body"] for r in agent_records]
    lines.append(f"- requests: {len(reqs)}; outcome: {json.dumps(seen)}")
    if not reqs:
        return lines
    first, last = reqs[0], reqs[-1]
    if name == "text":
        msgs = first["messages"]
        system = text_of(msgs[0]) if msgs[0]["role"] in ("system", "developer") else ""
        lines.append(f"- first message role: `{msgs[0]['role']}`; system prompt {len(system)} chars; "
                     f"AGENTS.md included: {'MARKER-AGENTS-MD' in json.dumps(msgs)}")
        lines.append(f"- system prompt starts: {system[:160]!r}")
        for tool, schema in tool_names(first).items():
            props = schema.get("properties", {})
            lines.append(f"  - tool `{tool}`: params {sorted(props)}; required {sorted(schema.get('required', []))}")
        lines.append(f"- request options: `{json.dumps(options(first), sort_keys=True)}`")
    elif name == "tools":
        for m in last["messages"][1:]:
            if m["role"] == "assistant" and m.get("tool_calls"):
                calls = [(c["function"]["name"], sorted(json.loads(c["function"]["arguments"] or "{}")))
                         for c in m["tool_calls"]]
                lines.append(f"  - assistant tool_calls {calls}; content {shape(m.get('content'))}")
            elif m["role"] == "tool":
                lines.append(f"  - tool result ({shape(m.get('content'))}): {text_of(m)[:140]!r}")
    elif name == "continue":
        roles = [m["role"] for m in last["messages"]]
        lines.append(f"- last request roles: {roles}")
    elif name == "resources":
        for i, body in enumerate(reqs):
            msgs = body["messages"]
            system = text_of(msgs[0]) if msgs[0]["role"] in ("system", "developer") else ""
            user = text_of([m for m in msgs if m["role"] == "user"][-1])
            lines.append(f"  - request {i}: skill listed {'MARKER-SKILL' in system}; "
                         f"template expanded {'MARKER-TEMPLATE review the.' in user}; user text {user[:80]!r}")
    elif name == "context":
        msgs = first["messages"]
        system = text_of(msgs[0]) if msgs[0]["role"] in ("system", "developer") else ""
        found = [m for m in ("HOME", "PARENT", "AGENTS", "OVERRIDE", "SYSTEM", "APPEND", "FLAG")
                 if f"MARKER-{m}" in system]
        lines.append(f"- in the system prompt: {found}; starts {system[:70]!r}")
    elif name == "image":
        user = [m for m in last["messages"] if m["role"] == "user"][-1]
        lines.append(f"- user content shape: {shape(user.get('content'))}")
    elif name == "thinking":
        lines.append(f"- last request options: `{json.dumps(options(last), sort_keys=True)}`")
    elif name == "compaction":
        for i, body in enumerate(reqs):
            msgs = body["messages"]
            mark = " **summary request**" if is_summary_request(body) else ""
            lines.append(f"  - #{i}: {len(msgs)} messages, ~{len(json.dumps(msgs)) // 4} tokens, "
                         f"tools {len(body.get('tools', []))}, max_tokens {body.get('max_tokens', body.get('max_completion_tokens'))}{mark}")
            if mark:
                lines.append(f"    - instruction: {text_of(msgs[-1])[:220]!r}")
                if msgs[0]["role"] in ("system", "developer"):
                    lines.append(f"    - system: {text_of(msgs[0])[:160]!r}")
    return lines


SCENARIOS = ["text", "tools", "continue", "image", "thinking", "compaction", "resources", "context", "tui",
             "failure", "cancel"]


def main():
    binary = pathlib.Path(sys.argv[1]).resolve()
    checkout = pathlib.Path(sys.argv[2] if len(sys.argv) > 2 else ROOT / ".workspace/pi").resolve()
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    failed = []
    report = ["# Pi vs vivy on the scripted gateway", ""]
    try:
        with tempfile.TemporaryDirectory(prefix="vivy compare ") as scratch:
            root = pathlib.Path(scratch)
            agents = [Pi(root, server.server_port, checkout), Vivy(root, server.server_port, binary)]
            for scenario in SCENARIOS:
                report += [f"## {scenario}", ""]
                for agent in agents:
                    capture = OUT / agent.name / f"{scenario}.jsonl"
                    capture.parent.mkdir(parents=True, exist_ok=True)
                    capture.write_text("", encoding="utf-8")
                    server.capture, server.scenario = capture, scenario
                    try:
                        seen = scenario_run(agent, scenario)
                    except Exception as error:  # report and go on
                        seen = {"error": repr(error)}
                        failed.append(f"{agent.name}/{scenario}")
                    found = records(capture)
                    if not found and scenario != "cancel":
                        failed.append(f"{agent.name}/{scenario} (no request)")
                    if any(not r["bearer"] for r in found):
                        failed.append(f"{agent.name}/{scenario} (no bearer key)")
                    report.append(f"### {agent.name}")
                    report += summarize(scenario, found, seen)
                    report.append("")
    finally:
        server.shutdown()
        server.server_close()
    text = "\n".join(report)
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "summary.md").write_text(text + "\n", encoding="utf-8")
    print(text)
    if failed:
        print("\nFAILED: " + ", ".join(failed), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()

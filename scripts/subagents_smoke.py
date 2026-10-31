"""Native Vivy delegation against a scripted gateway; no provider credentials.

Usage: python scripts/subagents_smoke.py <vivy.exe> [--tui]
TUI test dependencies: pyte==0.8.2, wcwidth==0.2.13, Windows pywinpty==3.1.0.
VIVY_SUBAGENT_EVIDENCE optionally receives terminal screen captures.
"""
import ctypes
import json
import os
import pathlib
import re
import shlex
import subprocess
import sys
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

from mock_gateway import Handler, text_of


def specification(description, prompt, profile="scout", cwd=None):
    return dict(description=description, prompt=prompt, profile=profile, **({"cwd": cwd} if cwd else {}))


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        with self.server.lock:
            self.server.records.append(body)
            assert len(self.server.records) < 150, "unexpected model loop"
        assert body["model"] == "org/model-1", body["model"]
        assert self.headers.get("authorization") == "Bearer test-key"
        messages = body["messages"]
        users = [text_of(message) for message in messages if message["role"] == "user"]
        system = "\n".join(text_of(message) for message in messages if message["role"] == "system")
        if "You are a subagent." in system:
            return self.child(body, users[-1], system)
        case = next((user for user in reversed(users) if "_CASE" in user), "")
        if "LATE_ROLE_CASE" in case:
            role = self.server.late_project / ".rig/agents/scout.md"
            role.parent.mkdir(parents=True, exist_ok=True)
            role.write_text("---\ntools: [write]\n---\nUNTRUSTED_LATE_ROLE", encoding="utf-8")
        last = messages[-1]
        if "PARALLEL_CASE" in case:
            if last["role"] == "tool":
                result = json.loads(text_of(last))
                assert [row["status"] for row in result["results"]] == ["completed", "completed"]
                assert "alpha" in result["results"][0]["answer"] and "beta" in result["results"][1]["answer"]
                self.server.parallel = result
                return self.reply(body, text="PARALLEL_ACCEPTED")
            return self.reply(body, tool="task_parallel", call={"tasks":[
                specification("Left scan", "CHILD_READ_A", cwd="left"),
                specification("Right scan", "CHILD_READ_B", cwd="right")]})
        if "CHAIN_CASE" in case:
            if last["role"] == "tool":
                result = json.loads(text_of(last))
                assert len(result["results"]) == 3 and all(row["status"] == "completed" for row in result["results"])
                return self.reply(body, text="CHAIN_ACCEPTED")
            return self.reply(body, tool="task_chain", call={"steps":[
                specification("Scout", "CHILD_SCOUT"),
                specification("Implement", "CHILD_WORKER use {previous}", "worker", "nested"),
                specification("Review", "CHILD_REVIEW use {previous}", "reviewer", "nested")]})
        if "ERROR_CASE" in case:
            if last["role"] == "tool":
                result = json.loads(text_of(last))
                assert [row["status"] for row in result["results"]] == ["completed", "failed"]
                assert "alpha" in result["results"][0]["answer"]
                assert "scripted child rejection" in result["results"][1]["answer"]
                return self.reply(body, text="PARTIAL_FAILURE_ACCEPTED")
            return self.reply(body, tool="task_parallel", call={"tasks":[
                specification("Good", "CHILD_READ_A", cwd="left"), specification("Bad", "CHILD_FAIL")]})
        if "LIMIT_CASE" in case:
            if last["role"] == "tool":
                assert "at most 8" in text_of(last)
                return self.reply(body, text="LIMIT_ACCEPTED")
            return self.reply(body, tool="task_parallel", call={"tasks":[specification(f"Task {i}", "CHILD_UNUSED") for i in range(9)]})
        if "CANCEL_CASE" in case:
            return self.reply(body, tool="task_parallel", call={"tasks":[
                specification(f"Blocking {i}", f"CHILD_BLOCK_{i}", "worker") for i in range(6)]})
        expected = "LATE_ROLE_DONE" if "LATE_ROLE_CASE" in case else "CHILD_SINGLE_DONE" if "SINGLE_CASE" in case else "ROLE_DONE"
        if any(expected in user for user in users):
            return self.reply(body, text="LATE_ROLE_ACCEPTED" if "LATE_ROLE_CASE" in case else "SINGLE_ACCEPTED" if "SINGLE_CASE" in case else "ROLE_ACCEPTED")
        if last["role"] == "tool":
            match = re.search(r"Started subagent `([^`]+)`", text_of(last))
            if match:
                return self.reply(body, tool="wait", call={"agent": match.group(1)})
            # The wait result and its associated report reach the next request.
            return self.reply(body, text="WAIT_FINISHED")
        if "LATE_ROLE_CASE" in case:
            return self.reply(body, tool="task", call=specification("Late role", "CHILD_LATE_ROLE"))
        return self.reply(body, tool="task", call=specification("Single investigation", "CHILD_SINGLE" if "SINGLE_CASE" in case else "CHILD_ROLE", "worker" if "SINGLE_CASE" in case else "custom"))

    def child(self, body, task, system):
        assert "PARENT_PRIVATE_CONTEXT" not in task, "parent conversation leaked"
        last = body["messages"][-1]
        offered = {tool["function"]["name"] for tool in body["tools"]}
        assert not {"task", "task_parallel", "task_chain"} & offered, offered
        if "CHILD_LATE_ROLE" in task:
            assert "UNTRUSTED_LATE_ROLE" not in system and "write" not in offered
            return self.reply(body, text="LATE_ROLE_DONE")
        if "CHILD_FAIL" in task:
            return self.reply_error(401, "scripted child rejection")
        if "CHILD_BLOCK_" in task:
            index = int(re.search(r"CHILD_BLOCK_(\d+)", task).group(1))
            code = f'import os,pathlib,time;pathlib.Path("cancel-{index}.pid").write_text(str(os.getpid()));time.sleep(300)'
            command = shlex.quote(sys.executable.replace("\\", "/")) + " -c " + shlex.quote(code)
            return self.reply(body, tool="shell", call={"command": command, "timeout_secs": 300})
        if "CHILD_WORKER" in task:
            assert "SCOUT_FOUND" in task
            if last["role"] == "tool":
                return self.reply(body, text="WORKER_WROTE result.txt")
            return self.reply(body, tool="write", call={"path":"result.txt","content":"implemented from scout alpha"})
        if "CHILD_REVIEW" in task:
            assert "WORKER_WROTE" in task
            if last["role"] == "tool":
                assert "implemented from scout alpha" in text_of(last)
                return self.reply(body, text="REVIEW_OK")
            return self.reply(body, tool="read", call={"path":"result.txt"})
        if "CHILD_ROLE" in task:
            assert self.server.expected_role in system
            assert ("PROJECT_ROLE" in system) == (self.server.expected_role == "PROJECT_ROLE")
            return self.reply(body, text="ROLE_DONE")
        if "CHILD_SINGLE" in task:
            return self.reply(body, text="CHILD_SINGLE_DONE")
        assert {"read", "search", "list"} <= offered and not {"write", "edit", "shell", "python_repl"} & offered
        if last["role"] == "tool":
            return self.reply(body, text=("SCOUT_FOUND " if "CHILD_SCOUT" in task else "FINDING ") + text_of(last))
        return self.reply(body, tool="read", call={"path":"note.txt"})


class View:
    def __init__(self, terminal):
        import pyte
        self.terminal = terminal
        self.screen = pyte.Screen(140, 45)
        self.stream = pyte.Stream(self.screen)
        self.position = 0
        self.captures = []

    def text(self):
        self.stream.feed(self.terminal.output[self.position:].decode("utf-8", "replace"))
        self.position = len(self.terminal.output)
        return "\n".join("".join(self.screen.buffer[y][x].data for x in range(self.screen.columns)) for y in range(self.screen.lines))

    def wait(self, name, predicate, timeout=25):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            self.terminal.read_for(0.15)
            text = self.text()
            if predicate(text):
                self.captures.append(f"--- {name} ---\n{text}")
                print("PASS:", name, flush=True)
                return
        raise AssertionError(name + "\n" + self.text())


def alive(pid):
    if os.name != "nt":
        try:
            os.kill(pid, 0)
            return True
        except ProcessLookupError:
            return False
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.OpenProcess.restype = ctypes.c_void_p
    kernel.GetExitCodeProcess.argtypes = [ctypes.c_void_p, ctypes.POINTER(ctypes.c_ulong)]
    kernel.CloseHandle.argtypes = [ctypes.c_void_p]
    handle = kernel.OpenProcess(0x1000, False, pid)
    if not handle:
        return False
    try:
        status = ctypes.c_ulong()
        return bool(kernel.GetExitCodeProcess(handle, ctypes.byref(status))) and status.value == 259
    finally:
        kernel.CloseHandle(handle)


def terminal_checks(binary, work, env, server):
    from tui import ENTER, Terminal
    terminal = Terminal([binary], work, env, rows=45, cols=140)
    view = View(terminal)
    try:
        view.wait("terminal starts", lambda text: "org/model-1" in text)
        terminal.type("/roles", enter=False)
        view.wait("role command input ready", lambda text: "/roles" in text)
        # Close completion before submitting the exact command; under ConPTY,
        # rapid synthetic typing can leave a stale completion selection.
        terminal.type(b"\x1b", enter=False)
        terminal.read_for(0.5)
        terminal.type(ENTER, enter=False)
        view.wait("built-in and user roles visible", lambda text: "scout:" in text and "custom:" in text)
        terminal.type("PARALLEL_CASE PARENT_PRIVATE_CONTEXT")
        view.wait("parallel card, Markdown answers and final reply", lambda text: "PARALLEL_ACCEPTED" in text and "Vivy Parallel" in text)
        assert '"results"' not in view.text()
        child = server.parallel["results"][0]["agent"]
        terminal.type("/agents " + child)
        view.wait("child conversation is inspectable", lambda text: "Left scan" in text and "note.txt" in text)
        terminal.type("/agents")
        view.wait("agent picker opens", lambda text: "Show an agent" in text)
        terminal.type(b"\x1b[A", enter=False)
        terminal.type(ENTER, enter=False)
        view.wait("return to parent conversation", lambda text: "PARALLEL_ACCEPTED" in text)
        terminal.type("/usage")
        view.wait("native usage includes children", lambda text: "subagents" in text.lower())
        terminal.type("CANCEL_CASE")
        view.wait("four running tasks and two queued", lambda text: len(list(work.glob("cancel-*.pid"))) == 4 and "Parallel: 0/6 completed" in text)
        pids = [int(path.read_text()) for path in work.glob("cancel-*.pid")]
        assert all(alive(pid) for pid in pids)
        terminal.type(b"\x1b", enter=False)  # Rig's running-turn stop key.
        view.wait("parent cancellation terminates shell descendants", lambda text: all(not alive(pid) for pid in pids))
        terminal.read_for(0.5)
        assert len(list(work.glob("cancel-*.pid"))) == 4, "queued work started after cancellation"
    finally:
        view.captures.append("--- final ---\n" + view.text())
        terminal.close()
        if evidence := os.environ.get("VIVY_SUBAGENT_EVIDENCE"):
            path = pathlib.Path(evidence)
            path.mkdir(parents=True, exist_ok=True)
            (path / "terminal.txt").write_text("\n\n".join(view.captures), encoding="utf-8")


def main():
    binary = str(pathlib.Path(sys.argv[1]).resolve())
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    server.records, server.lock = [], threading.Lock()
    server.expected_role, server.parallel = "USER_ROLE", None
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="vivy subagents ") as scratch:
            root = pathlib.Path(scratch)
            home, work = root / "home", root / "work"
            home.mkdir()
            work.mkdir()
            for name, value in [("left","alpha"),("right","beta"),("nested", "nested")]:
                (work / name).mkdir()
                (work / name / "note.txt").write_text(value)
            (work / "note.txt").write_text("alpha")
            (home / "agents").mkdir()
            (home / "agents/custom.md").write_text("---\ntools: [read]\n---\nUSER_ROLE", encoding="utf-8")
            (work / ".rig/agents").mkdir(parents=True)
            (work / ".rig/agents/custom.md").write_text("---\ntools: [read]\n---\nPROJECT_ROLE", encoding="utf-8")
            (home / "settings.toml").write_text(
                f'[gateway]\nbase_url = "http://127.0.0.1:{server.server_port}/v1"\nmodel = "org/model-1"\n'
                'context_window = 131072\ninput_price = 0.5\noutput_price = 1.5\n[trust]\ndefault = "never"\n', encoding="utf-8")
            env = dict(os.environ, RIG_HOME=str(home), RIG_SECRET_GATEWAY="test-key")
            for case, expected in [("SINGLE", "SINGLE_ACCEPTED"),("PARALLEL","PARALLEL_ACCEPTED"),("CHAIN","CHAIN_ACCEPTED"),("ERROR","PARTIAL_FAILURE_ACCEPTED"),("LIMIT","LIMIT_ACCEPTED"),("ROLE","ROLE_ACCEPTED")]:
                result = subprocess.run([binary,"-p",case+"_CASE PARENT_PRIVATE_CONTEXT"], cwd=work, env=env, text=True, encoding="utf-8", capture_output=True, timeout=45)
                assert result.returncode == 0 and expected in result.stdout, (case, result.returncode, result.stdout, result.stderr)
                assert "Agent-loop systems use the same data with no order" not in result.stderr, result.stderr
                print("PASS:", case.lower(), flush=True)
            assert (work / "nested/result.txt").read_text() == "implemented from scout alpha"
            server.expected_role = "PROJECT_ROLE"
            result = subprocess.run([binary,"--approve","-p","ROLE_CASE"], cwd=work, env=env, text=True, encoding="utf-8", capture_output=True, timeout=45)
            assert result.returncode == 0 and "ROLE_ACCEPTED" in result.stdout, (result.stdout, result.stderr)
            print("PASS: trusted project role overrides user role", flush=True)
            server.expected_role = "USER_ROLE"
            # Start with no protected project resources. Add a role only after
            # startup's trust decision, under an explicit refusal.
            with tempfile.TemporaryDirectory(prefix="late trust ", dir=pathlib.Path(binary).parent) as late:
                server.late_project = pathlib.Path(late)
                result = subprocess.run([binary,"--no-approve","-p","LATE_ROLE_CASE"], cwd=late, env=env, text=True, encoding="utf-8", capture_output=True, timeout=45)
                assert result.returncode == 0 and "LATE_ROLE_ACCEPTED" in result.stdout, (result.returncode, result.stdout, result.stderr)
                print("PASS: late project roles do not bypass refused trust", flush=True)
            if "--tui" in sys.argv:
                terminal_checks(binary, work, env, server)
            for path in home.rglob("*"):
                if path.is_file():
                    assert b"test-key" not in path.read_bytes(), path
            print("PASS: raw model ID, child tool restrictions, secret filtering", flush=True)
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main()

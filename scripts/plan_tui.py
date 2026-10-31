"""Plan mode acceptance against a scripted loopback gateway and a real TUI.

Usage: python scripts/plan_tui.py <path-to-vivy>
Test-only dependencies: pyte==0.8.2 and wcwidth==0.2.13;
Windows also needs pywinpty==3.1.0.
No provider credentials are used. An optional VIVY_PLAN_EVIDENCE directory
receives terminal screens on success or failure.
"""
import json
import os
import pathlib
import sys
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

import pyte
from mock_gateway import Handler, text_of
from tui import ENTER, Terminal

PLAN = "# Small change\n" + "\n".join(
    f"Step {n:03}: reuse the existing implementation and verify it." for n in range(1, 81)
) + "\nLAST-PLAN-ITEM"


def planning(body):
    return any("You are in Plan mode." in text_of(m) for m in body["messages"] if m["role"] == "system")


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        self.server.records.append(body)
        last = body["messages"][-1]
        if last["role"] == "tool":
            output = text_of(last)
            if "Plan approved." in output:
                return self.reply(body, call={"path": "approved.txt", "content": "approved"}, tool="write")
            if "User feedback:" in output:
                return self.reply(body, text="FEEDBACK-SEEN: " + output.split("User feedback:", 1)[1])
            return self.reply(body, text="EXECUTION-FINISHED")
        user = [text_of(m) for m in body["messages"] if m["role"] == "user"][-1]
        # A cancelled prompt may be recalled together with the next typed
        # message; detect the test task in that ordinary user message too.
        if "review " in user:
            return self.reply(body, call={"plan": PLAN}, tool="exit_plan_mode")
        return self.reply(body, text="SCOPE-QUESTION: Which small change?")


def skip_initialization(terminal, view):
    """Exercise the real skip controls before unrelated terminal scenarios."""
    deadline = time.monotonic() + 25
    while time.monotonic() < deadline:
        terminal.read_for(.2)
        text = view.text()
        if 'Initialization checks complete' in text:
            return
        if 'Set up ' in text and 'Skip for this project' in text:
            terminal.type(b'\x1b[B\r', enter=False)
            terminal.read_for(.5)
    raise AssertionError('Initialization did not finish\n' + view.text())


class View:
    def __init__(self, terminal):
        self.terminal = terminal
        self.screen = pyte.Screen(120, 40)
        self.stream = pyte.Stream(self.screen)
        self.position = 0
        self.captures = []

    def text(self):
        data = self.terminal.output[self.position:]
        self.position = len(self.terminal.output)
        self.stream.feed(data.decode("utf-8", "replace"))
        # ConPTY can leave a wide-character continuation at the left edge.
        # Read cells directly: pyte's display formatter indexes that empty cell.
        return "\n".join("".join(self.screen.buffer[y][x].data
                                 for x in range(self.screen.columns))
                         for y in range(self.screen.lines))

    def wait(self, name, predicate, timeout=20):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            self.terminal.read_for(0.2)
            text = self.text()
            if predicate(text):
                self.captures.append(f"--- {name} ---\n{text}\n")
                print("PASS", name, flush=True)
                return
        self.captures.append(f"--- FAILED {name} ---\n{self.text()}\n")
        raise AssertionError(name + "\n" + self.text())


def command(terminal, view, text):
    """Submit outside a review after the exact editor text is rendered."""
    terminal.type(text, enter=False)
    view.wait("command input ready: " + text, lambda screen:
              any(line.strip() == "› " + text for line in screen.splitlines()))
    # Enter may otherwise accept a stale slash-command completion when
    # synthetic ConPTY input arrives faster than the terminal redraw.
    terminal.type(b"\x1b", enter=False)
    terminal.read_for(0.5)
    terminal.type(ENTER, enter=False)


def main(binary):
    evidence = os.environ.get("VIVY_PLAN_EVIDENCE")
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    server.records = []
    threading.Thread(target=server.serve_forever, daemon=True).start()
    terminal = view = None
    scratch = tempfile.TemporaryDirectory(prefix="vivy plan ")
    try:
        root = pathlib.Path(scratch.name)
        home, work = root / "home", root / "work"
        home.mkdir()
        work.mkdir()
        (home / "settings.toml").write_text(
            f'[gateway]\nbase_url = "http://127.0.0.1:{server.server_port}/v1"\n'
            'model = "org/model-1"\ncontext_window = 64000\n'
            '[trust]\ndefault = "never"\n', encoding="utf-8")
        env = dict(os.environ, RIG_HOME=str(home), RIG_SECRET_GATEWAY="test-key")
        terminal = Terminal([binary], work, env)
        view = View(terminal)
        skip_initialization(terminal, view)
        view.wait("terminal starts", lambda text: "org/model-1" in text)

        command(terminal, view, "/plan investigate")
        view.wait("ordinary answer keeps PLAN without review", lambda text:
                  "SCOPE-QUESTION" in text and "PLAN" in text and "Approve and execute" not in text)
        assert planning(server.records[-1])

        terminal.type("review approve")
        view.wait("plan review and state marker", lambda text:
                  "Approve and execute" in text and "REVIEW" in text)
        assert "LAST-PLAN-ITEM" not in view.text(), "long review should initially show its beginning"
        for _ in range(90):
            terminal.type(b"\x1b[1;2B", enter=False)  # Shift+Down scrolls the body.
            terminal.read_for(0.01)
        view.wait("full plan can be scrolled", lambda text: "LAST-PLAN-ITEM" in text)
        terminal.type(ENTER, enter=False)
        view.wait("approval executes and clears the marker", lambda text:
                  "EXECUTION-FINISHED" in text and "PLAN" not in text.splitlines()[-1])
        assert (work / "approved.txt").read_text() == "approved"
        assert not planning(server.records[-1])
        tools = [[t["function"]["name"] for t in record["tools"]] for record in server.records]
        assert all(names == tools[0] for names in tools), "Plan must not alter the tool catalog"

        command(terminal, view, "/plan review feedback")
        view.wait("next plan review", lambda text: "Approve and execute" in text)
        terminal.type("Use fewer components")
        view.wait("feedback reaches model and keeps PLAN", lambda text:
                  "FEEDBACK-SEEN" in text and "PLAN" in text.splitlines()[-1])
        assert "Use fewer components" in json.dumps(server.records[-1])
        assert planning(server.records[-1])

        terminal.type("review cancel")
        view.wait("cancellable review", lambda text: "Approve and execute" in text)
        count = len(server.records)
        terminal.type(b"\x1b", enter=False)
        view.wait("Esc cancels review without leaving Plan", lambda text:
                  "Plan review cancelled" in text and "Approve and execute" not in text)
        assert len(server.records) == count
        terminal.type("review withdraw")
        view.wait("review accepts manual exit", lambda text: "Approve and execute" in text)
        count = len(server.records)
        terminal.type("/plan off")
        view.wait("manual exit withdraws review", lambda text:
                  "Plan mode off" in text and "Approve and execute" not in text)
        assert len(server.records) == count
        command(terminal, view, "/plan")
        view.wait("Plan can be reentered", lambda text: "PLAN" in text.splitlines()[-1])
        command(terminal, view, "/plan off")
        view.wait("manual exit clears PLAN", lambda text:
                  "Plan mode off" in text and "PLAN" not in text.splitlines()[-1])

        command(terminal, view, "/plan")
        view.wait("enter before restart", lambda text: "PLAN" in text.splitlines()[-1])
        command(terminal, view, "/quit")
        terminal.read_for(2)
        terminal.close()
        session_id = next((home / "sessions").iterdir()).name
        terminal = Terminal([binary, "--resume", session_id], work, env)
        view.terminal, view.position = terminal, 0
        view.screen.reset()
        view.wait("resumed terminal restores PLAN", lambda text: "PLAN" in text.splitlines()[-1])
        command(terminal, view, "/plan off")
        view.wait("resumed Plan can exit", lambda text: "Plan mode off" in text)
        command(terminal, view, "/quit")
        terminal.read_for(1)
    finally:
        if evidence and view:
            dest = pathlib.Path(evidence)
            dest.mkdir(parents=True, exist_ok=True)
            (dest / "plan-screens.txt").write_text("\n".join(view.captures), encoding="utf-8")
            (dest / "plan-requests.json").write_text(json.dumps(server.records, ensure_ascii=False, indent=2), encoding="utf-8")
            if terminal:
                (dest / "plan-terminal.txt").write_bytes(terminal.output)
        if terminal:
            terminal.close()
        server.shutdown()
        server.server_close()
        scratch.cleanup()


if __name__ == "__main__":
    main(str(pathlib.Path(sys.argv[1]).resolve()))

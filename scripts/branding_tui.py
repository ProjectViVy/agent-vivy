"""Compact terminal acceptance on a scripted gateway; no provider credentials.

Usage: python scripts/branding_tui.py <vivy.exe>
Test-only dependencies are the same as plan_tui.py. VIVY_TUI_EVIDENCE optionally
receives synthetic screens and terminal streams for visual inspection.
"""
import json
from contextlib import ExitStack
import os
import pathlib
import sys
import tempfile
import threading
from http.server import ThreadingHTTPServer

import pyte
from mock_gateway import Handler, text_of
from plan_tui import View as PlanView, command, skip_initialization
from tui import ENTER, Terminal


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        self.server.records.append(body)
        system = "\n".join(text_of(m) for m in body["messages"] if m["role"] == "system")
        assert "You are Vivy, a coding agent" in system
        assert "You are rig," not in system
        last = body["messages"][-1]
        user = next(text_of(m) for m in reversed(body["messages"]) if m["role"] == "user")
        if last["role"] == "tool":
            return self.reply(body, text="TOOL-FINISHED\n\n**Result:** all output is retained.")
        if "ERROR_CASE" in user:
            return self.reply(body, tool="shell", call={"command": "printf 'VISIBLE-CAUSE\\n'; exit 7"})
        if "TOOLS_CASE" in user:
            return self.reply(body, tool="shell", call={"command": "printf 'FIRST-OUTPUT\\n'; printf 'LAST-OUTPUT\\n'"})
        if "BLOCK_CASE" in user:
            return self.reply(body, tool="shell", call={"command": "printf 'WAITING-TO-CANCEL\\n'; sleep 300", "timeout_secs": 300})
        return self.reply(body, text="REPLY-DONE\n\nA compact response with `code`.")


class View(PlanView):
    def __init__(self, terminal, cols, rows):
        super().__init__(terminal)
        self.screen = pyte.Screen(cols, rows)
        self.stream = pyte.Stream(self.screen)

    def capture(self, root, name):
        if root:
            root.mkdir(parents=True, exist_ok=True)
            (root / (name + ".txt")).write_text(self.text(), encoding="utf-8")
            (root / (name + ".ansi")).write_bytes(self.terminal.output)

    def resize(self, rows, cols):
        """Resizes the screen first, so the redrawn bytes are read at it."""
        self.screen.resize(rows, cols)
        self.terminal.resize(rows, cols)
        self.terminal.read_for(0.5)


def scenario(binary, cols, rows, evidence):
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    server.records = []
    threading.Thread(target=server.serve_forever, daemon=True).start()
    terminal = view = None
    try:
        with ExitStack() as resources:
            scratch = resources.enter_context(tempfile.TemporaryDirectory(prefix="vivy compact "))
            root = pathlib.Path(scratch)
            home, work = root / "home", root / "project with spaces"
            home.mkdir()
            work.mkdir()
            (home / "settings.toml").write_text(
                f'[gateway]\nbase_url = "http://127.0.0.1:{server.server_port}/v1"\n'
                'model = "org/model-1"\ndialect = "openrouter"\n'
                'context_window = 64000\nreasoning = ["low", "high"]\n'
                '[trust]\ndefault = "never"\n', encoding="utf-8")
            env = dict(os.environ, RIG_HOME=str(home), RIG_SECRET_GATEWAY="test-key")
            terminal = Terminal([binary], work, env, rows=rows, cols=cols)
            resources.callback(terminal.close)
            view = View(terminal, cols, rows)
            skip_initialization(terminal, view)
            view.wait("compact startup", lambda text: "Vivy (v0.1.0)" in text and "org/model-1" in text)
            initial = view.text()
            assert "rig v" not in initial and "█▙" not in initial
            assert "Ask Vivy to do anything" in initial
            view.capture(evidence, "startup")

            # A resize must redraw at the new size. Windows' ConPTY reports the
            # console buffer's size in the resize event, which stays as the app
            # found it, so the view asks the terminal for the size it has now
            # and draws again when the frame it drew is not that size.
            smaller = (max(16, rows - 8), max(40, cols - 16))
            view.resize(*smaller)
            view.wait("footer follows a shrink", lambda text:
                      "idle" in text.splitlines()[-1] and "org/model-1" in text.splitlines()[-1])
            view.capture(evidence, "shrunk")
            roomier = (rows + 2, cols + 10)
            view.resize(*roomier)
            view.wait("footer follows a grow", lambda text:
                      "idle" in text.splitlines()[-1] and "org/model-1" in text.splitlines()[-1])
            view.capture(evidence, "grown")
            view.resize(rows, cols)
            view.wait("footer follows the original size", lambda text:
                      "idle" in text.splitlines()[-1] and "org/model-1" in text.splitlines()[-1])
            view.capture(evidence, "restored")

            command(terminal, view, "/plan")
            view.wait("plan marker", lambda text: "PLAN" in text.splitlines()[-1])
            terminal.type("检查代码", enter=False)
            view.wait("draft retains footer", lambda text: "检查代码" in text and "PLAN" in text.splitlines()[-1])
            view.capture(evidence, "draft")
            terminal.type(b"\x15", enter=False)  # Ctrl+U clears the single-line draft.
            view.wait("cleared draft restores placeholder", lambda text: "Ask Vivy" in text)
            command(terminal, view, "/plan off")
            view.wait("plan left", lambda text: "PLAN" not in text.splitlines()[-1])

            # Backslash+Enter is a native multiline continuation on terminals
            # whose input protocol cannot distinguish Shift+Enter.
            terminal.type("检查代码\\")
            terminal.type("second line", enter=False)
            view.wait("multiline input", lambda text: "检查代码" in text and "second line" in text)
            assert not server.records, "continuation must not submit the first line"
            view.capture(evidence, "multiline")
            terminal.type(ENTER, enter=False)
            view.wait("streamed markdown reply", lambda text: "REPLY-DONE" in text)
            user = next(m for m in server.records[0]["messages"] if m["role"] == "user")
            assert text_of(user) == "检查代码\nsecond line"

            terminal.type("TOOLS_CASE")
            view.wait("full shell output", lambda text:
                      any(line.strip() == "FIRST-OUTPUT" for line in text.splitlines()) and
                      any(line.strip() == "LAST-OUTPUT" for line in text.splitlines()) and "TOOL-FINISHED" in text)
            view.capture(evidence, "tools")

            terminal.type("ERROR_CASE")
            view.wait("tool failure cause", lambda text:
                      any(line.strip() == "VISIBLE-CAUSE" for line in text.splitlines()) and
                      any(line.lstrip().startswith("×") and "exit 7" in line for line in text.splitlines()))
            view.capture(evidence, "error")

            terminal.type("BLOCK_CASE")
            view.wait("running turn", lambda text:
                      any(line.strip() == "WAITING-TO-CANCEL" for line in text.splitlines()) and "Esc stop" in text)
            view.capture(evidence, "running")
            terminal.type(b"\x1b", enter=False)
            view.wait("cancel returns to idle", lambda text: "idle" in text.splitlines()[-1] and "Esc stop" not in text)
            view.capture(evidence, "cancelled")
            raw = terminal.output.decode("utf-8", "replace")
            assert "\x1b]2;Vivy ·" in raw
            assert "\x1b]2;rig ·" not in raw
            assert server.records
            command(terminal, view, "/quit")
            # pywinpty can leave its output reader waiting after the process
            # has exited; use its process status rather than output EOF.
            eof = not terminal.read_for(2)
            assert not terminal.process.isalive() if os.name == "nt" else eof
        print(f"PASS: compact TUI {cols}x{rows}, branding, input, output, failure and cancellation", flush=True)
    finally:
        if terminal:
            if evidence:
                view.capture(evidence, "last")
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    binary = str(pathlib.Path(sys.argv[1]).resolve())
    evidence = os.environ.get("VIVY_TUI_EVIDENCE")
    for cols, rows in [(120, 40), (50, 20)]:
        destination = pathlib.Path(evidence) / f"{cols}x{rows}" if evidence else None
        scenario(binary, cols, rows, destination)

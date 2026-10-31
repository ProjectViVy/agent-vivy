"""A small pseudo-terminal driver (pywinpty is test-only on Windows).

Terminal views redraw with cursor moves, so `seen` matches text with all
whitespace removed, in what arrived since a mark.
"""
import os
import queue
import re
import select
import signal
import subprocess
import threading
import time

if os.name != "nt":
    import fcntl
    import pty
    import struct
    import termios

ALT_UP = b"\x1b[1;3A"
ENTER = b"\r"
CTRL_C = b"\x03"
CTRL_D = b"\x04"


def plain(raw):
    """`raw` terminal output as text, escape sequences removed."""
    text = raw.decode("utf-8", "replace")
    return re.sub(r"\x1b\[[0-9;?<>=]*[a-zA-Z~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[()=>][A-Z0-9]?", "", text)


def squeeze(text):
    return re.sub(r"\s+", "", text)


class Terminal:
    def __init__(self, argv, cwd, env, rows=40, cols=120):
        self.output = b""
        if os.name == "nt":
            from winpty import PtyProcess
            # Match the product's Windows launcher: ConPTY inherits the
            # console code page, and crossterm emits UTF-8.
            argv = ["cmd.exe", "/d", "/c", "chcp 65001>nul & " + subprocess.list2cmdline(argv)]
            self.process = PtyProcess.spawn(argv, cwd=str(cwd), env=env, dimensions=(rows, cols))
            self.pid = self.process.pid
            self.chunks = queue.Queue()
            threading.Thread(target=self._pump, daemon=True).start()
            return
        self.pid, self.fd = pty.fork()
        if self.pid == 0:
            os.chdir(cwd)
            os.execvpe(argv[0], argv, env)
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))

    def _pump(self):
        try:
            while True:
                self.chunks.put(self.process.read(65536).encode("utf-8"))
        except (EOFError, OSError):
            self.chunks.put(None)

    def mark(self):
        return len(self.output)

    def since(self, mark):
        return plain(self.output[mark:])

    def read_for(self, seconds, until=None, since=0):
        """Reads for `seconds`, or until `until` shows after `since`."""
        end = time.time() + seconds
        while time.time() < end:
            try:
                if os.name == "nt":
                    data = self.chunks.get(timeout=0.2)
                    if data is None:
                        return False
                    self.output += data
                elif select.select([self.fd], [], [], 0.2)[0]:
                    self.output += os.read(self.fd, 65536)
            except queue.Empty:
                pass
            except OSError:
                return False
            if until is not None and squeeze(until) in squeeze(self.since(since)):
                return True
        return until is None

    def type(self, text, enter=True):
        self._write(text)
        if enter:
            time.sleep(0.3)
            self._write(ENTER)

    def resize(self, rows, cols):
        """Gives the terminal a new size, as a window drag does."""
        if os.name == "nt":
            self.process.setwinsize(rows, cols)
            return
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))

    def _write(self, text):
        if os.name == "nt":
            self.process.write(text.decode("utf-8") if isinstance(text, bytes) else text)
        else:
            os.write(self.fd, text.encode() if isinstance(text, str) else text)

    def close(self):
        if os.name == "nt":
            self.process.close(force=True)
            return
        for key in (CTRL_C, CTRL_C, CTRL_D, CTRL_D):
            try:
                os.write(self.fd, key)
            except OSError:
                break
            time.sleep(0.3)
        try:
            os.kill(self.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            os.waitpid(self.pid, 0)
        except ChildProcessError:
            pass
        os.close(self.fd)

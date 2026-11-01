"""Native pipe boundary tests. All homes and sessions are disposable."""
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import tempfile
import threading
import unittest

BINARY = Path(sys.argv.pop(1)).resolve()


class Client:
    def __init__(self, root, session=None):
        env = {k: v for k, v in os.environ.items()
               if not k.startswith(("RIG_", "VIVY_BOOTSTRAP_"))}
        env["RIG_HOME"] = str(root / "home")
        if session:
            env["RIG_SESSION"] = session
        self.process = subprocess.Popen([str(BINARY), "--diva"], cwd=root,
                                        env=env, stdin=subprocess.PIPE,
                                        stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.frames = queue.Queue()
        self.errors = bytearray()
        self.sequence = 0
        self.instance = None
        def read():
            try:
                for line in self.process.stdout:
                    try:
                        self.frames.put(json.loads(line))
                    except Exception as error:
                        self.frames.put(error)
            finally:
                self.frames.put(None)
        def diagnostics():
            for chunk in iter(lambda: self.process.stderr.read(4096), b""):
                self.errors.extend(chunk)
        self.readers = [threading.Thread(target=read), threading.Thread(target=diagnostics)]
        for reader in self.readers:
            reader.start()

    def receive(self):
        frame = self.frames.get(timeout=20)
        assert isinstance(frame, dict), (frame, self.errors.decode(errors="replace"))
        assert frame["seq"] == self.sequence + 1, frame
        assert frame["v"] == 1, frame
        if self.instance is not None:
            assert frame["instance"] == self.instance, frame
        self.instance = frame["instance"]
        self.sequence = frame["seq"]
        return frame

    def write(self, value):
        self.process.stdin.write(value)
        self.process.stdin.flush()

    def close(self):
        if self.process.poll() is None:
            self.process.kill()
        self.process.wait(timeout=10)
        for reader in self.readers:
            reader.join(timeout=10)
            assert not reader.is_alive()
        for pipe in (self.process.stdin, self.process.stdout, self.process.stderr):
            pipe.close()


class TransportTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="diva-transport-")
        self.root = Path(self.directory.name)
        self.clients = []
        self.addCleanup(self.directory.cleanup)
        self.addCleanup(self.close_clients)

    def close_clients(self):
        for client in self.clients:
            client.close()

    def start(self, session=None):
        client = Client(self.root, session)
        self.clients.append(client)
        ready = client.receive()
        self.assertEqual(ready["type"], "ready")
        return client, ready

    def test_eof_exits_and_session_reopens(self):
        first, ready = self.start()
        first.process.stdin.close()
        self.assertEqual(first.process.wait(timeout=15), 0)
        second, restored = self.start(ready["data"]["session"])
        self.assertEqual(restored["data"]["session"], ready["data"]["session"])
        self.assertNotEqual(restored["instance"], ready["instance"])
        second.process.stdin.close()
        self.assertEqual(second.process.wait(timeout=15), 0)

    def test_malformed_frames_recover_without_echoing_input(self):
        client, _ = self.start()
        for raw, code in [(b'\xff\n', "invalid_frame"),
                          (b'{"secret":"DO_NOT_ECHO"\n', "invalid_request"),
                          (b'{"v":1,"id":"extra","method":"snapshot","secret":"DO_NOT_ECHO"}\n', "invalid_request")]:
            client.write(raw)
            frame = client.receive()
            self.assertEqual(frame["data"]["code"], code)
            self.assertNotIn("DO_NOT_ECHO", json.dumps(frame))
        client.write(json.dumps({"v": 1, "id": "多行\n请求", "method": "snapshot"}, ensure_ascii=False).encode() + b'\n')
        self.assertEqual(client.receive()["id"], "多行\n请求")

    def test_oversize_frame_reports_and_exits(self):
        client, _ = self.start()
        client.write(b'x' * (1024 * 1024 + 1))
        self.assertEqual(client.receive()["data"]["code"], "invalid_frame")
        self.assertEqual(client.process.wait(timeout=15), 0)

    def test_unterminated_frame_is_not_executed(self):
        client, _ = self.start()
        client.write(b'{"v":1,"id":"partial","method":"snapshot"}')
        client.process.stdin.close()
        frame = client.receive()
        self.assertEqual(frame["type"], "error")
        self.assertEqual(frame["data"]["code"], "invalid_frame")
        self.assertEqual(client.process.wait(timeout=15), 0)


if __name__ == "__main__":
    unittest.main()

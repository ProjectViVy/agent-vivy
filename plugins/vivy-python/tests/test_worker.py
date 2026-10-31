"""Offline tests of the actual embedded worker, without the agent or a provider."""
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest


class WorkerTests(unittest.TestCase):
    def setUp(self):
        self.artifacts = tempfile.TemporaryDirectory()
        self.addCleanup(self.artifacts.cleanup)
        worker = pathlib.Path(__file__).parents[1] / "src" / "worker.py"
        self.process = subprocess.Popen(
            [sys.executable, "-u", str(worker)], stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, encoding="utf-8", env={**os.environ, "VIVY_WORKBENCH_ARTIFACTS": self.artifacts.name},
        )
        self.assertEqual(json.loads(self.process.stdout.readline())["event"], "ready")

    def tearDown(self):
        self.process.stdin.close()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait()
        self.process.stdout.close()
        self.process.stderr.close()

    def execute(self, code):
        self.process.stdin.write(json.dumps({"type": "execute", "id": "one", "code": code}) + "\n")
        self.process.stdin.flush()
        events = []
        while True:
            line = self.process.stdout.readline()
            self.assertTrue(line, "worker exited before answering")
            event = json.loads(line)
            events.append(event)
            if event["event"] == "done":
                return events

    def test_variables_await_and_last_expression(self):
        self.execute("answer = 40")
        events = self.execute("import asyncio\nawait asyncio.sleep(0)\nanswer + 2")
        self.assertTrue(any(e.get("text") == "42" for e in events))

    def test_raw_and_subprocess_output_cannot_corrupt_the_protocol(self):
        events = self.execute("import os,subprocess,sys\nprint('你好')\nos.write(1,b'raw\\n')\nsubprocess.run([sys.executable,'-c',\"print('child')\"])\nNone")
        text = "".join(e.get("text", "") for e in events)
        self.assertIn("你好", text)
        self.assertIn("raw", text)
        self.assertIn("child", text)

    def test_exception_preserves_existing_variables(self):
        self.execute("answer = 42")
        events = self.execute("raise ValueError('expected')")
        self.assertTrue(any(e["event"] == "error" and e.get("ename") == "ValueError" for e in events))
        self.assertTrue(any(e.get("text") == "42" for e in self.execute("answer")))

    def test_printed_output_precedes_the_last_expression(self):
        events = self.execute("print('before'); 'after'")
        printed = next(i for i, e in enumerate(events) if e["event"] == "stdout")
        result = next(i for i, e in enumerate(events) if e["event"] == "result")
        self.assertLess(printed, result)

    def test_unicode_result_fits_the_host_frame_budget(self):
        events = self.execute("s = chr(0x1F600) * 600000; s")
        for event in events:
            self.assertLess(len(json.dumps(event, ensure_ascii=False).encode("utf-8")), 2 * 1024 * 1024)
        self.assertTrue(any(e.get("text") == "600000" for e in self.execute("len(s)")))

    def test_short_progress_is_delivered_before_the_cell_finishes(self):
        with tempfile.TemporaryDirectory(prefix="vivy progress ") as directory:
            release = pathlib.Path(directory) / "release"
            code = f"import pathlib,time\nprint('early',flush=True)\nstarted=time.monotonic()\nwhile not pathlib.Path({str(release)!r}).exists():\n if time.monotonic()-started>2: raise RuntimeError('progress was buffered')\n time.sleep(.01)"
            self.process.stdin.write(json.dumps({"type": "execute", "id": "two", "code": code}) + "\n")
            self.process.stdin.flush()
            first = json.loads(self.process.stdout.readline())
            self.assertEqual(first["event"], "stdout")
            release.touch()
            while True:
                event = json.loads(self.process.stdout.readline())
                if event["event"] == "done":
                    self.assertEqual(event["status"], "ok")
                    break


if __name__ == "__main__":
    unittest.main()

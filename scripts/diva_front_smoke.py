"""Native desktop transport acceptance; uses only a fresh temporary home."""
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import tempfile
import threading


def main():
    binary = Path(sys.argv[1]).resolve()
    with tempfile.TemporaryDirectory(prefix="diva-front-") as directory:
        root = Path(directory)
        env = {k: v for k, v in os.environ.items()
               if not k.startswith(("RIG_", "VIVY_BOOTSTRAP_"))}
        env["RIG_HOME"] = str(root / "home")
        process = subprocess.Popen([str(binary), "--diva"], cwd=root, env=env,
                                   stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, text=True, encoding="utf-8")
        lines = queue.Queue()
        errors = []
        def read():
            for line in process.stdout:
                lines.put(json.loads(line))
            lines.put(None)
        threading.Thread(target=read, daemon=True).start()
        threading.Thread(target=lambda: errors.append(process.stderr.read()), daemon=True).start()
        sequence = 0
        def receive():
            nonlocal sequence
            event = lines.get(timeout=20)
            assert event is not None, errors
            assert event["v"] == 1 and event["seq"] > sequence, event
            sequence = event["seq"]
            if event["type"] not in ("ready", "reply", "error"):
                return receive()
            return event
        def request(value):
            process.stdin.write(json.dumps(value, ensure_ascii=False) + "\n")
            process.stdin.flush()
            return receive()
        try:
            hello = receive()
            assert hello["type"] == "ready" and hello["data"]["session"], hello
            assert hello["data"]["plugins"], hello
            bad = request({"v": 999, "id": "bad", "method": "snapshot"})
            assert bad["type"] == "error" and bad["id"] == "bad", bad
            assert bad["data"]["code"] == "incompatible_version", bad
            unknown = request({"v": 1, "id": "unknown", "method": "arbitrary_shell"})
            assert unknown["type"] == "error", unknown
            assert unknown["data"]["code"] == "unsupported_operation", unknown
            oversized_id = request({"v": 1, "id": "x" * 257, "method": "snapshot"})
            assert oversized_id["id"] is None, oversized_id
            state = request({"v": 1, "id": "状态", "method": "snapshot"})
            assert state["type"] == "reply" and state["id"] == "状态", state
            assert state["instance"] == hello["instance"], state
            bye = request({"v": 1, "id": "exit", "method": "shutdown"})
            assert bye["type"] == "reply", bye
            assert process.wait(timeout=15) == 0, errors
            print("PASS: native ready, session, inventory, version/operation rejection, Unicode snapshot, shutdown")
        finally:
            if process.poll() is None:
                process.kill()
            process.wait(timeout=10)


if __name__ == "__main__":
    main()

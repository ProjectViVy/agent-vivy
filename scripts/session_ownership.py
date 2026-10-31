"""Exercise session ownership through the real launcher and generated executable.

Usage: python scripts/session_ownership.py <vivy.exe>
Only disposable sessions and the loopback gateway are used.
"""
import os
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

from durable_probe import probe_environment
from mock_gateway import Handler, text_of


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        self.server.records.append(body)
        prompt = next(text_of(m) for m in reversed(body["messages"]) if m["role"] == "user")
        if prompt == "D0_HOLD" and not self.server.waiting.is_set():
            allow_disconnect = self.server.allow_disconnect
            self.server.waiting.set()
            assert self.server.release.wait(25)
            try:
                return self.reply(body, text="D0_HOLD_DONE")
            except ConnectionError:
                if not allow_disconnect:
                    raise
                return  # The process-loss case deliberately closes this socket.
        self.reply(body, text="D0_SIMPLE_DONE")


def main():
    builder = str(Path(sys.argv[1]).resolve())
    species = str(Path(os.environ["CARGO_TARGET_DIR"]) / "debug" / (
        "coding-agent.exe" if os.name == "nt" else "coding-agent"))
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    server.records = []
    server.waiting = threading.Event()
    server.release = threading.Event()
    server.allow_disconnect = False
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="vivy-ownership-") as temporary:
            root = Path(temporary)
            home = root / "home"
            home.mkdir()
            (home / "settings.toml").write_text(
                f'[gateway]\nbase_url = "http://127.0.0.1:{server.server_port}/v1"\n'
                'model = "ownership/model"\n[trust]\ndefault = "never"\n', encoding="utf-8")
            env = dict(probe_environment(os.environ), RIG_HOME=str(home), RIG_SECRET_GATEWAY="synthetic")

            def run(binary, *args, environment=None):
                return subprocess.run([binary, *args], cwd=root, env=environment or env,
                    capture_output=True, text=True, encoding="utf-8", timeout=20)

            def accepted(result):
                assert result.returncode == 0 and "D0_SIMPLE_DONE" in result.stdout, (
                    result.returncode, result.stdout, result.stderr)

            accepted(run(builder, "-p", "D0_SIMPLE"))
            session = next((home / "sessions").iterdir())
            direct_env = dict(env, RIG_SESSION=session.name)

            for direct, terminate in [(False, False), (True, True), (False, True)]:
                server.allow_disconnect = terminate
                server.waiting.clear()
                server.release.clear()
                executable = species if direct else builder
                args = [] if direct else ["--resume", session.name]
                owner = subprocess.Popen([executable, *args, "-p", "D0_HOLD"], cwd=root,
                    env=direct_env if direct else env, stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE, text=True, encoding="utf-8")
                try:
                    assert server.waiting.wait(15), "owner did not reach gateway"
                    before = len(server.records)
                    saved = {p.name: p.read_bytes() for p in (session / "logs").iterdir()}
                    sessions = set((home / "sessions").iterdir())
                    for binary, arguments, environment in [
                        (species, ["-p", "D0_SIMPLE"], direct_env),
                        (species, ["-p", "D0_SIMPLE"], dict(direct_env, RIG_DESCRIBE="")),
                        (builder, ["--resume", session.name, "-p", "D0_SIMPLE"], env),
                        (builder, ["--fork", session.name, "-p", "D0_SIMPLE"], env),
                        (species, ["-p", "D0_SIMPLE"], dict(direct_env,
                            RIG_RUN=str(home / "run" / str(owner.pid)), RIG_LAUNCHER=builder)),
                    ]:
                        result = run(binary, *arguments, environment=environment)
                        assert result.returncode != 0, (direct, binary, arguments, result.stdout)
                        assert "already running" in result.stderr or "handoff" in result.stderr, result.stderr
                        assert len(server.records) == before
                        assert saved == {p.name: p.read_bytes() for p in (session / "logs").iterdir()}
                        assert set((home / "sessions").iterdir()) == sessions, "failed fork left a target"
                    if terminate:
                        owner.kill()
                    server.release.set()
                    stdout, stderr = owner.communicate(timeout=15)
                    if not terminate:
                        assert owner.returncode == 0 and "D0_HOLD_DONE" in stdout, (stdout, stderr)
                finally:
                    server.release.set()
                    if owner.poll() is None:
                        owner.kill()
                        owner.communicate(timeout=15)
                # A killed launcher may need a short interval for Rig's existing
                # launcher watcher to exit the child and release execution locks.
                deadline = time.monotonic() + 10
                while True:
                    result = run(builder, "--resume", session.name, "-p", "D0_SIMPLE")
                    if result.returncode == 0:
                        accepted(result)
                        break
                    assert "already running" in result.stderr and time.monotonic() < deadline, result.stderr
                    time.sleep(0.05)
            accepted(run(species, "-p", "D0_SIMPLE", environment=direct_env))
            # Rig tools may retain only the launcher executable for nested
            # builds. Without SESSION/RUN this is a standalone invocation.
            accepted(run(species, "-p", "D0_SIMPLE", environment=dict(env, RIG_LAUNCHER=builder)))
            accepted(run(builder, "--fork", session.name, "-p", "D0_SIMPLE"))
            saved_sessions = set((home / "sessions").iterdir())
            accepted(run(builder, "--no-session", "-p", "D0_SIMPLE"))
            assert saved_sessions == set((home / "sessions").iterdir())
            sentinel = root / "outside-description"
            sentinel.write_text("unchanged", encoding="utf-8")
            poisoned = dict(env, RIG_DESCRIBE=str(sentinel), RIG_RUN=str(root / "outside-run"),
                RIG_SESSION="1793440000-9999", RIG_NOTICE="OUTSIDE_NOTICE", RIG_BUILD_FAILURE="OUTSIDE_BUILD",
                RIG_PROCESS_LEDGER=str(root / "outside-groups"), RIG_LAUNCHER="outside-launcher")
            accepted(run(builder, "--resume", session.name, "-p", "D0_SIMPLE", environment=poisoned))
            assert sentinel.read_text(encoding="utf-8") == "unchanged"
            assert not (root / "outside-run").exists() and not (root / "outside-groups").exists()
            assert not any("OUTSIDE_NOTICE" in json.dumps(record) or "OUTSIDE_BUILD" in json.dumps(record)
                           for record in server.records)
            print("PASS: launcher/direct ownership, refused concurrent fork, invalid handoff, exit and process-loss release")
    finally:
        server.release.set()
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main()

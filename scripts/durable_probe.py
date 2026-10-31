"""D0 synthetic coding-path probe. Exit 2 means reproduced persistence gaps.

Usage: python scripts/durable_probe.py <vivy.exe>
Uses only loopback, disposable sessions and an explicitly compiled probe plugin.
Rust characterizations run too; reports remain under .workspace/d0.
"""
import json
import os
import shutil
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
from http.server import ThreadingHTTPServer

from mock_gateway import Handler, text_of
from subagents_optional import variant, ROOT


def probe_environment(environment):
    """Keep toolchain settings, discard enclosing Rig process control/secrets."""
    return {k: v for k, v in environment.items()
            if not k.upper().startswith("RIG_") and k.upper() != "VIVY_D0_TRACE"}


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        self.server.records.append(body)
        prompt = next(text_of(m) for m in reversed(body["messages"]) if m["role"] == "user")
        if prompt == "D0_HOLD":
            self.server.waiting.set()
            assert self.server.release.wait(25), "ownership probe deadline"
            return self.reply(body, text="D0_HOLD_DONE")
        if prompt == "D0_SIMPLE":
            return self.reply(body, text="D0_SIMPLE_DONE")
        if body["messages"][-1]["role"] == "tool":
            assert "D0_FILE_EVIDENCE" in text_of(body["messages"][-1])
            self.reply(body, text="D0_TASK_DONE")
        else:
            self.reply(body, tool="read", call={"path": "sample.txt"})


def analyze(events):
    """Require observed writes for the exact operation, not arbitrary flushes."""
    prepared = {}
    written = {}
    released = {}
    starts = []
    kinds = set()
    for event in events:
        assert event["durable"] is False, "probe must not advertise disk receipts"
        kind = event["event"]
        if kind == "checkpoint_prepared":
            cp = event["checkpoint"]
            prepared[(event["agent"], cp["operation"])] = cp
        elif kind == "append_observed":
            for cp in event["checkpoints"]:
                written[(event["log"], cp["operation"])] = cp
        elif kind == "write_gate_released":
            cp = event["checkpoint"]
            key = (event["agent"], cp["operation"])
            assert written.get(key) == cp == prepared.get(key), event
            released[key] = cp
        elif kind == "started":
            key = (event["agent"], event["operation"])
            assert key in released, f"operation started without its observed checkpoint: {event}"
            assert released[key] == prepared.get(key) == written.get(key), event
            kinds.add(released.pop(key)["kind"])
            starts.append(event)
    assert kinds == {"input", "model", "tool"}, kinds
    assert len([e for e in prepared.values() if e["kind"] == "tool"]) == 1
    return {"observed_starts": len(starts), "kinds": sorted(kinds),
            "trace_events": len(events), "disk_receipts": 0}


def entry_paths(builder, binary, root, home, server):
    """Characterize launch/resume with disposable copies, never production data."""
    baseline = Path(os.environ["CARGO_TARGET_DIR"]) / "debug" / (
        "coding-agent.exe" if os.name == "nt" else "coding-agent")
    env = probe_environment(os.environ)
    env.update(RIG_HOME=str(home), RIG_SECRET_GATEWAY="synthetic-d0")
    source = next((home / "sessions").iterdir())
    (source / "directory").write_text(str(root), encoding="utf-8")

    def copy(name):
        dest = home / "sessions" / name
        shutil.copytree(source, dest)
        return dest

    def run(executable, *args, session=None):
        return subprocess.run([str(executable), *args], cwd=root,
            env=env if session is None else dict(env, RIG_SESSION=session.name),
            text=True, encoding="utf-8", capture_output=True, timeout=30)

    # An unsupported Rig log format is rejected by both entry points, before
    # requests and without modifying the logical store. This does not prove a
    # future Vivy durable-format guard or a required-plugin check.
    incompatible = copy("1793440000-9001")
    for path in (incompatible / "logs").glob("*.jsonl"):
        rows = path.read_text(encoding="utf-8").splitlines()
        header = json.loads(rows[0])
        header["format"] = "rig-session/999"
        rows[0] = json.dumps(header)
        path.write_text("\n".join(rows) + "\n", encoding="utf-8")
    snapshot = {str(p.relative_to(incompatible)): p.read_bytes()
                for folder in ["logs", "files"] for p in (incompatible / folder).rglob("*") if p.is_file()}
    count = len(server.records)
    for executable, args, session in [
        (builder, ["--resume", incompatible.name, "-p", "D0_SIMPLE"], None),
        (binary, ["-p", "D0_SIMPLE"], incompatible),
    ]:
        result = run(executable, *args, session=session)
        assert result.returncode != 0, result.stdout
        assert "rig-session/999" in result.stderr, result.stderr
        assert count == len(server.records)
        assert snapshot == {str(p.relative_to(incompatible)): p.read_bytes()
            for folder in ["logs", "files"] for p in (incompatible / folder).rglob("*") if p.is_file()}

    # The baseline recipe still accepts the experimental state it cannot load.
    missing = copy("1793440000-9002")
    result = run(builder, "--resume", missing.name, "-p", "D0_SIMPLE")
    assert result.returncode == 0 and "D0_SIMPLE_DONE" in result.stdout, result.stderr
    forked = run(builder, "--fork", source.name, "-p", "D0_SIMPLE")
    assert forked.returncode == 0 and "D0_SIMPLE_DONE" in forked.stdout, forked.stderr

    malformed = copy("1793440000-9003")
    changed = False
    for path in (malformed / "logs").glob("*.jsonl"):
        rows = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines()]
        for row in rows:
            if row.get("key") == "vivy::d0_probe/1":
                row["value"] = {"revision": "invalid"}
                changed = True
        path.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")
    assert changed
    result = run(binary, "-p", "D0_SIMPLE", session=malformed)
    assert result.returncode == 0 and "D0_SIMPLE_DONE" in result.stdout, result.stderr

    # Real launcher ownership: hold the first process at the loopback model.
    owned = copy("1793440000-9004")
    first = subprocess.Popen([builder, "--resume", owned.name, "-p", "D0_HOLD"],
        cwd=root, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        text=True, encoding="utf-8")
    try:
        assert server.waiting.wait(20), "first launcher did not reach the gateway"
        count = len(server.records)
        second = run(builder, "--resume", owned.name, "-p", "D0_SIMPLE")
        assert second.returncode != 0 and "already running" in second.stderr, second.stderr
        assert len(server.records) == count
        # Generated mains now acquire ownership before constructing the app.
        direct = run(baseline, "-p", "D0_SIMPLE", session=owned)
        assert len(server.records) == count, direct.stderr
        assert direct.returncode != 0 and "already running" in direct.stderr, direct.stderr
    finally:
        server.release.set()
        try:
            first.communicate(timeout=15)
        except subprocess.TimeoutExpired:
            first.kill()
            first.communicate(timeout=10)
    return {"unknown_rig_format": "refused unchanged by launcher and species",
            "missing_probe_plugin_resume_and_fork": "accepted (gap)",
            "malformed_probe_state": "accepted (gap)",
            "second_launcher": "refused before model request",
            "direct_species_with_same_session": "refused before model request"}


def main():
    builder = str(Path(sys.argv[1]).resolve())
    environment = probe_environment(os.environ)
    output = ROOT / ".workspace/d0"
    output.mkdir(parents=True, exist_ok=True)
    # Do not leave a previous run's conclusion looking like current evidence.
    (output / "report.json").write_text(json.dumps({"status": "running"}), encoding="utf-8")
    with (output / "rust-probes.log").open("w", encoding="utf-8") as log:
        subprocess.run(["cargo", "test", "--manifest-path",
                        str(ROOT / "plugins/vivy-durable/Cargo.toml"),
                        "--features", "d0-probe", "--locked"],
                       check=True, cwd=ROOT, env=environment, stdout=log, stderr=subprocess.STDOUT)
    extra = f"""
  - name: d0-probe
    crate: vivy-durable
    path: {json.dumps(str(ROOT / 'plugins/vivy-durable'))}
    plugins:
      - vivy_durable::D0ProbePlugin
    features:
      vivy-durable: [d0-probe]
"""
    binary = variant(builder, "d0-durable-probe",
                     ["python", "subagents", "branch-summary", "session-commands"],
                     extra=extra, env=environment)
    graph = subprocess.check_output(["cargo", "tree", "--manifest-path",
        str(ROOT / ".generated/coding-agent/Cargo.toml"), "--locked", "-e", "normal"],
        text=True, encoding="utf-8", cwd=ROOT, env=environment)
    assert "vivy-durable" not in graph, "legacy recipe compiled the experimental plugin"
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    server.records = []
    server.waiting = threading.Event()
    server.release = threading.Event()
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with (tempfile.TemporaryDirectory(prefix="vivy-d0-") as temporary,
              tempfile.TemporaryDirectory(prefix="vivy-d0-sentinel-") as sentinel):
            root = Path(temporary)
            home = root / "home"
            home.mkdir()
            (root / "sample.txt").write_text("D0_FILE_EVIDENCE\n", encoding="utf-8")
            (home / "settings.toml").write_text(
                f'[gateway]\nbase_url = "http://127.0.0.1:{server.server_port}/v1"\n'
                'model = "d0/model"\n[trust]\ndefault = "never"\n', encoding="utf-8")
            path = root / "trace.jsonl"
            outside = Path(sentinel)
            (outside / "description").write_text("unchanged sentinel", encoding="utf-8")
            before = {p.name: p.read_bytes() for p in outside.iterdir()}
            poisoned = dict(os.environ, RIG_RUN=str(outside), RIG_SESSION="1793440000-9999",
                RIG_START="resumed", RIG_NOTICE="OUTSIDE_NOTICE", RIG_BUILD_FAILURE="OUTSIDE_BUILD",
                RIG_PROCESS_LEDGER=str(outside / "groups"), RIG_DESCRIBE=str(outside / "description"),
                RIG_LAUNCHER=str(outside / "launcher"))
            result = subprocess.run([str(binary), "-p", "Read sample.txt"],
                cwd=root, env=dict(probe_environment(poisoned), RIG_HOME=str(home),
                    RIG_SECRET_GATEWAY="synthetic-d0", VIVY_D0_TRACE=str(path)),
                text=True, encoding="utf-8", capture_output=True, timeout=45)
            assert before == {p.name: p.read_bytes() for p in outside.iterdir()}, "inherited Rig state escaped fixture"
            assert result.returncode == 0 and "D0_TASK_DONE" in result.stdout, (
                result.returncode, result.stdout, result.stderr)
            assert len(server.records) == 2, len(server.records)
            assert not any("OUTSIDE_BUILD" in json.dumps(record) or "OUTSIDE_NOTICE" in json.dumps(record)
                           for record in server.records), "inherited launcher input contaminated task"
            events = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines()]
            stats = analyze(events)
            assert "on the whole world" not in result.stderr, result.stderr
            (output / "coding-trace.json").write_text(json.dumps(events, indent=2), encoding="utf-8")
            (output / "coding.stderr").write_text(result.stderr, encoding="utf-8")
            entries = entry_paths(builder, binary, root, home, server)
        report = {
            "rig_revision": "fc556f982d501eea9d7167398a19386e4884e776",
            "status": "blocked",
            "meaning": "D0 investigation ran; persistence acceptance has not passed.",
            "coding_path": stats,
            "entry_paths": entries,
            "fixture_isolation": "inherited Rig controls stripped; outside sentinels unchanged",
            "supported": ["multi-pass holds", "exact observed-write matching",
                          "request/route/generation invalidation", "native stop and CarryOn opt-in",
                          "launcher and generated species session ownership"],
            "gaps": [
                {"id": "sync", "evidence": "source inspection at the pinned revision",
                 "finding": "DirStore append uses write_all without sync; no disk receipt was exercised."},
                {"id": "result", "evidence": "Rust characterization and coding trace",
                 "finding": "Review precedes final redaction; settlement is not guarded by a committed result."},
                {"id": "required-state", "evidence": "Rust characterization and assembled entry checks",
                 "finding": "Invalid or unregistered required state does not refuse the session."},
                {"id": "serialization", "evidence": "Rust characterization",
                 "finding": "A failed saved component can be omitted while the other log write succeeds."},
            ],
            "unverified": ["power-loss durability", "full D1 crash matrix",
                           "real provider", "production cancellation generations",
                           "saved model-result reinjection",
                           "opaque endpoint changes under the same provider/model identity"],
        }
        (output / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
        print(json.dumps(report, indent=2), flush=True)
        return 2
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    raise SystemExit(main())

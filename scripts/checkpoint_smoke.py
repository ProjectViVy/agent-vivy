"""Synthetic D0 acceptance of the opt-in local Rig checkpoint build on Windows.

Only disposable session data is used. This does not test a real provider or
OS/power-loss durability. Usage: checkpoint_smoke.py <vivy> <rig-checkout>
"""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import threading
import shutil
import base64
import hashlib
from http.server import ThreadingHTTPServer

from durable_probe import Gateway, probe_environment

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / ".workspace/d0-d1"


def entry_checks(builder, rig, recipe_path, binary, root, home, env, server):
    source=next((home/"sessions").iterdir())
    baseline=Path(env["CARGO_TARGET_DIR"])/"debug/coding-agent.exe"
    assert baseline.is_file(),"build the ordinary recipe before the entry checks"

    def snapshot(path):
        return {str(p.relative_to(path)):p.read_bytes() for folder in ["logs","files"]
                for p in (path/folder).rglob("*") if p.is_file()}

    def refused(command, session):
        before=snapshot(session);count=len(server.records)
        result=subprocess.run(command,cwd=root,env=dict(env,RIG_SESSION=session.name),capture_output=True,text=True,encoding="utf-8",timeout=30)
        assert result.returncode!=0,(command,result.stdout,result.stderr)
        assert snapshot(session)==before,"rejected entry changed session data"
        assert len(server.records)==count,"rejected entry called the provider"

    refused([str(baseline),"-p","D0_SIMPLE"],source)
    refused([builder,"run",str(recipe_path),"--rig-path",rig,"--","--fork",source.name,"-p","D0_SIMPLE"],source)
    # Valid physical groups, invalid required component, plus a torn tail: the
    # schema refusal must precede any recovery-file creation or truncation.
    bad=home/"sessions/1793450000-9901"
    shutil.copytree(source,bad)
    for path in (bad/"logs").glob("*.jsonl"):
        lines=path.read_bytes().splitlines();output=[lines[0]];start=0
        for line in lines[1:]:
            group=json.loads(line);logical=[]
            for row in base64.b64decode(group["data"]).splitlines():
                record=json.loads(row)
                if record.get("key")=="vivy.checkpoint_probe.v1":record["value"]={"incompatible":True}
                logical.append(json.dumps(record,separators=(",",":"),ensure_ascii=False).encode())
            data=b"\n".join(logical)+b"\n"
            group["start"]=start;start+=len(data);group["data"]=base64.b64encode(data).decode()
            payload=["rig-checkpoint/1",group["lineage"],group["sequence"],group["start"],group["data"]]
            group["sha256"]=hashlib.sha256(json.dumps(payload,separators=(",",":"),ensure_ascii=False).encode()).hexdigest()
            output.append(json.dumps(group,separators=(",",":"),ensure_ascii=False).encode())
        path.write_bytes(b"\n".join(output)+b"\n{torn-tail")
    refused([str(binary),"-p","D0_SIMPLE"],bad)
    refused([builder,"run",str(recipe_path),"--rig-path",rig,"--","--resume",bad.name,"-p","D0_SIMPLE"],bad)
    corrupt=home/"sessions/1793450000-9902"
    shutil.copytree(source,corrupt)
    path=next((corrupt/"logs").glob("*.jsonl"))
    lines=path.read_bytes().splitlines();last=json.loads(lines[-1]);last["sha256"]="0"*64
    path.write_bytes(b"\n".join(lines[:-1]+[json.dumps(last).encode()])+b"\n")
    refused([str(binary),"-p","D0_SIMPLE"],corrupt)
    return ["missing plugin","fork refused before copy","invalid required state before torn-tail repair: direct + launcher","complete checksum corruption"]


def recipe():
    original = (ROOT / "recipes/coding-agent.yaml").read_text(encoding="utf-8")
    header, components = original.split("components:\n", 1)
    selected = {"prompt", "providers", "models", "coding-tools", "print"}
    pieces = re.split(r"(?=^  - name:)", components, flags=re.MULTILINE)
    text = header.replace("name: coding-agent", "name: checkpoint-probe") + "components:\n"
    text += "".join(p for p in pieces if any(p.startswith(f"  - name: {n}\n") for n in selected))
    text = re.sub(r"path: ../plugins/([^\s]+)", lambda m: "path: " + json.dumps(str(ROOT / "plugins" / m[1])), text)
    text += f'''  - name: checkpoint-probe
    crate: vivy-durable
    path: {json.dumps(str(ROOT / "plugins/vivy-durable"))}
    plugins: [vivy_durable::CheckpointProbePlugin]
    features:
      vivy-durable: [checkpoint-probe]
'''
    path = OUT / "checkpoint-probe.yaml"
    path.write_text(text, encoding="utf-8")
    return path


def main():
    builder, rig = map(lambda p: str(Path(p).resolve()), sys.argv[1:3])
    OUT.mkdir(parents=True, exist_ok=True)
    path = recipe()
    environment = probe_environment(os.environ)
    with (OUT / "checkpoint-build.log").open("w", encoding="utf-8") as log:
        subprocess.run([builder, "build", str(path), "--rig-path", rig], cwd=ROOT,
                       env=environment, stdout=log, stderr=subprocess.STDOUT, check=True)
    binary = Path(environment["CARGO_TARGET_DIR"]) / "rig-dev/debug/checkpoint-probe.exe"
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    server.records = []
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="vivy checkpoint smoke ") as directory:
            root = Path(directory)
            home = root / "home"
            home.mkdir()
            (root / "sample.txt").write_text("D0_FILE_EVIDENCE", encoding="utf-8")
            (home / "settings.toml").write_text(f'[gateway]\nbase_url="http://127.0.0.1:{server.server_port}/v1"\nmodel="model"\n', encoding="utf-8")
            trace = root / "trace.jsonl"
            env = dict(environment, RIG_HOME=str(home), RIG_SECRET_GATEWAY="synthetic",
                       VIVY_CHECKPOINT_TRACE=str(trace))
            result = subprocess.run([builder, "run", str(path), "--rig-path", rig, "--", "-p", "D0_READ"],
                                    cwd=root, env=env, capture_output=True, text=True, encoding="utf-8", timeout=40)
            (OUT / "checkpoint-run.log").write_text(result.stdout + result.stderr, encoding="utf-8")
            assert result.returncode == 0 and "D0_TASK_DONE" in result.stdout, (result.returncode, result.stdout, result.stderr)
            events = [json.loads(line) for line in trace.read_text(encoding="utf-8").splitlines()]
            (OUT / "checkpoint-trace.json").write_text(json.dumps(events, indent=2), encoding="utf-8")
            released = set()
            starts = 0
            for event in events:
                if event["event"] == "intent_released":
                    released.add(event["operation"])
                elif event["event"] in {"started_input", "started_model", "started_tool"}:
                    assert event["operation"] in released, event
                    starts += 1
            logs = list((home / "sessions").glob("*/logs/*.jsonl"))
            assert logs and all(p.read_bytes().startswith(b'{"format":"rig-checkpoint/1"}\n') for p in logs)
            assert len(server.records) == 2 and starts >= 3
            checks=entry_checks(builder,rig,path,binary,root,home,env,server)
            report = dict(status="passed", model_requests=2, starts=starts, trace_events=len(events),
                          physical_logs=len(logs),entry_checks=checks, scope="synthetic process path; no power-loss guarantee")
            (OUT / "checkpoint-report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
            print(json.dumps(report), flush=True)
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main()

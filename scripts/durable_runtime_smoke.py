"""Actual D1 executable process-crash matrix, using disposable loopback fixtures.

Usage: durable_runtime_smoke.py <vivy.exe> <local-rig>
The temporary recipe selects diagnostic fault hooks at compilation. The checked-in
persistent recipe does not include them. No real provider or power-loss claim.
"""
import base64
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer

from durable_probe import probe_environment
from mock_gateway import Handler, text_of
from subagents_smoke import alive

ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/".workspace/d0-d1"


class Gateway(Handler):
    def do_POST(self):
        body=json.loads(self.rfile.read(int(self.headers["content-length"])))
        self.server.requests.append(body)
        names={tool["function"]["name"] for tool in body["tools"]}
        assert not names & {"task","task_parallel","task_chain","message","wait"},names
        if body["messages"][-1]["role"]=="tool":
            return self.reply(body,text="D1_DONE")
        prompt=next(text_of(m) for m in reversed(body["messages"]) if m["role"]=="user")
        if "D1_CANCEL" in prompt:
            code="import os,pathlib,time;pathlib.Path('worker.pid').write_text(str(os.getpid()));time.sleep(300)"
            if "PYTHON" in prompt:
                return self.reply(body,tool="python_repl",call={"code":code,"timeout_secs":600})
            return self.reply(body,tool="shell",call={"command":shlex.quote(sys.executable)+" -c "+shlex.quote(code),"timeout_secs":600})
        return self.reply(body,tool="shell",call={"command":"printf 'EFFECT\\n' >> effects.txt"})


def state(session):
    values=[]
    for path in (session/"logs").glob("*.jsonl"):
        for line in path.read_bytes().splitlines()[1:]:
            group=json.loads(line)
            for raw in base64.b64decode(group["data"]).splitlines():
                record=json.loads(raw)
                if record.get("type")=="state" and record.get("key")=="vivy.durable.v1":
                    values.append(record["value"])
    assert values,"no committed durable state"
    return values[-1]


def wait_paused(process,trace,deadline):
    while time.monotonic()<deadline:
        if trace.exists():
            for line in trace.read_bytes().splitlines():
                try: event=json.loads(line)
                except json.JSONDecodeError: continue
                if event["paused"]: return event
        if process.poll() is not None: raise AssertionError(f"process exited before fault hook: {process.returncode}")
        time.sleep(.025)
    raise AssertionError("fault hook deadline")


def cancellation(binary,env,server,kind):
    with tempfile.TemporaryDirectory(prefix="vivy D1 stop ") as directory:
        root=Path(directory);home=root/"home";home.mkdir()
        (home/"settings.toml").write_text(f'[gateway]\nbase_url="http://127.0.0.1:{server.server_port}/v1"\nmodel="model"\n[trust]\ndefault="never"\n',encoding="utf-8")
        session=home/"sessions"/"1793450000-9300"
        environment=dict(env,RIG_HOME=str(home),RIG_SESSION=session.name,RIG_SECRET_GATEWAY="synthetic-d1")
        start=len(server.requests)
        with (OUT/f"cancel-{kind}.log").open("w",encoding="utf-8") as log:
            process=subprocess.Popen([str(binary),"--json"],cwd=root,env=environment,stdin=subprocess.PIPE,stdout=log,stderr=subprocess.STDOUT,text=True,encoding="utf-8")
            try:
                def send(value):
                    process.stdin.write(json.dumps(dict(v=1,**value))+"\n");process.stdin.flush()
                send(dict(type="send",text=f"D1_CANCEL {kind}"))
                deadline=time.monotonic()+45
                while not (root/"worker.pid").exists():
                    assert process.poll() is None and time.monotonic()<deadline,"worker did not start"
                    time.sleep(.025)
                pid=int((root/"worker.pid").read_text())
                assert alive(pid)
                send(dict(type="stop"))
                deadline=time.monotonic()+15
                while True:
                    try: final=state(session)
                    except (AssertionError,json.JSONDecodeError): final=None
                    if final and final["tasks"][0]["status"]=="cancelled" and not alive(pid): break
                    assert process.poll() is None and time.monotonic()<deadline,"stop did not commit and clean up worker"
                    time.sleep(.025)
                process.stdin.close();process.wait(timeout=15)
                assert process.returncode==0
                assert len(server.requests)-start==1,"model advanced after cancellation"
                result=subprocess.run([str(binary),"-p"],cwd=root,env=environment,stdout=log,stderr=subprocess.STDOUT,timeout=15)
                assert len(server.requests)-start==1,"cancelled work restarted"
                assert state(session)["tasks"][0]["status"]=="cancelled"
                return dict(stage="cancel:"+kind,status="passed",worker_stopped=True,restart_exit=result.returncode)
            finally:
                if process.poll() is None: process.kill();process.wait(timeout=10)


def main():
    builder,rig=map(lambda p:str(Path(p).resolve()),sys.argv[1:3])
    env=probe_environment(os.environ)
    source=(ROOT/"recipes/persistent-agent.yaml").read_text(encoding="utf-8")
    # Keep this matrix on the original D1 contract after the main recipe adopts
    # D2. D2 projection/maintenance has its own executable acceptance script.
    source=re.sub(r"  - name: persona\n.*?(?=  - name: print)","",source,flags=re.S)
    source=source.replace(", vivy_durable::AssistantContextPlugin","").replace("vivy-durable: [assistant-context]","vivy-durable: [runtime]").replace("vivy-durable: [wake]","vivy-durable: [runtime]").replace("vivy-durable: [autodream]","vivy-durable: [runtime]")
    source=source.replace("name: persistent-agent","name: persistent-matrix").replace("vivy-durable: [runtime]","vivy-durable: [runtime, checkpoint-probe]")
    source=re.sub(r"path: ../plugins/([^\s]+)",lambda m:"path: "+json.dumps(str(ROOT/"plugins"/m[1])),source)
    recipe=OUT/"persistent-matrix.yaml"
    recipe.write_text(source,encoding="utf-8")
    with (OUT/"matrix-build.log").open("w",encoding="utf-8") as log:
        subprocess.run([builder,"build",str(recipe),"--rig-path",rig],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
    binary=Path(env["CARGO_TARGET_DIR"])/"rig-dev/debug/persistent-matrix.exe"
    server=ThreadingHTTPServer(("127.0.0.1",0),Gateway)
    server.requests=[]
    threading.Thread(target=server.serve_forever,daemon=True).start()
    report=[]
    stages=["before_intent:model","after_intent:model","after_effect:model","after_result:model","before_intent:tool","after_intent:tool","after_effect:tool","after_result:tool","after_completion"]
    try:
        for index,stage in enumerate(stages):
            with tempfile.TemporaryDirectory(prefix="vivy D1 crash ") as directory:
                root=Path(directory);home=root/"home";home.mkdir()
                (home/"settings.toml").write_text(f'[gateway]\nbase_url="http://127.0.0.1:{server.server_port}/v1"\nmodel="model"\n[trust]\ndefault="never"\n',encoding="utf-8")
                session=home/"sessions"/f"1793450000-{9100+index}"
                trace=root/"trace.jsonl"
                environment=dict(env,RIG_HOME=str(home),RIG_SESSION=session.name,RIG_SECRET_GATEWAY="synthetic-d1",VIVY_DURABLE_TRACE=str(trace),VIVY_DURABLE_PAUSE=stage)
                start=len(server.requests);began=time.monotonic()
                with (OUT/f"matrix-{index}.log").open("w",encoding="utf-8") as log:
                    process=subprocess.Popen([str(binary),"-p","D1_ACT"],cwd=root,env=environment,stdout=log,stderr=subprocess.STDOUT)
                    try:
                        wait_paused(process,trace,time.monotonic()+45)
                        if index==0:
                            before={p.name:p.read_bytes() for p in (session/"logs").glob("*.jsonl")}
                            duplicate=subprocess.run([str(binary),"-p","duplicate writer"],cwd=root,env=environment,capture_output=True,text=True,encoding="utf-8",timeout=10)
                            assert duplicate.returncode!=0 and "already running" in duplicate.stderr
                            assert before=={p.name:p.read_bytes() for p in (session/"logs").glob("*.jsonl")}
                    finally:
                        if process.poll() is None: process.kill()
                        process.wait(timeout=10)
                    del environment["VIVY_DURABLE_PAUSE"]
                    (OUT/f"matrix-{index}-before.json").write_text(json.dumps(state(session),indent=2),encoding="utf-8")
                    restart_at=time.monotonic()
                    result=subprocess.run([str(binary),"-p"],cwd=root,env=environment,stdout=log,stderr=subprocess.STDOUT,timeout=45)
                    restart_seconds=time.monotonic()-restart_at
                final=state(session)
                (OUT/f"matrix-{index}-after.json").write_text(json.dumps(final,indent=2),encoding="utf-8")
                effects=(root/"effects.txt").read_text().count("EFFECT") if (root/"effects.txt").exists() else 0
                uncertain=stage in {"after_intent:tool","after_effect:tool"}
                expected_effects=0 if stage=="after_intent:tool" else 1
                assert effects==expected_effects,(stage,effects)
                assert any(o["phase"]=="unknown" for o in final["operations"])==uncertain,(stage,final)
                assert len(final["tasks"])==1,(stage,final)
                assert final["tasks"][0]["status"]==("suspended" if uncertain else "completed"),(stage,final)
                requests=len(server.requests)-start
                assert requests==(3 if stage=="after_effect:model" else 2),(stage,requests)
                logs=list((session/"logs").glob("*.jsonl"))
                report.append(dict(stage=stage,status="passed",effects=effects,model_requests=requests,restart_exit=result.returncode,
                                   restart_seconds=round(restart_seconds,3),log_bytes=sum(p.stat().st_size for p in logs),
                                   groups=sum(len(p.read_bytes().splitlines())-1 for p in logs),elapsed_seconds=round(time.monotonic()-began,3)))
                (OUT/"matrix-report.json").write_text(json.dumps(report,indent=2),encoding="utf-8")
                print(json.dumps(report[-1]),flush=True)
        for kind in ("SHELL","PYTHON"):
            report.append(cancellation(binary,env,server,kind))
            (OUT/"matrix-report.json").write_text(json.dumps(report,indent=2),encoding="utf-8")
            print(json.dumps(report[-1]),flush=True)
    finally:
        server.shutdown();server.server_close()


if __name__=="__main__": main()

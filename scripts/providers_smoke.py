"""Both provider wires against loopback fixtures; no real credentials or providers.

Usage: CARGO_TARGET_DIR=<build directory> python scripts/providers_smoke.py <vivy>
"""
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import threading
import tomllib
from http.server import ThreadingHTTPServer
from mock_gateway import Handler, text_of


class Gateway(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        messages_wire = self.path.endswith("/v1/messages")
        self.server.records.append((self.path, body, dict(self.headers)))
        prompt = next((text_of(m) for m in reversed(body["messages"]) if m["role"] == "user"
            and not any(p.get("type") == "tool_result" for p in m.get("content", []) if isinstance(p, dict))), "")
        if "FAIL_WIRE" in prompt:
            return self.reply_error(401, "fixture authentication rejection")
        if "FAIL_ECHO" in prompt:
            return self.reply_error(401, "fixture rejected " + (self.headers.get("authorization") or self.headers.get("x-api-key") or ""))
        if "FAIL_STREAM_ECHO" in prompt:
            self.send_response(200)
            self.send_header("content-type","text/event-stream")
            self.end_headers()
            failure={"type":"error","error":{"type":"authentication_error","code":"invalid_api_key","message":"fixture rejected "+(self.headers.get("authorization") or self.headers.get("x-api-key") or "")}}
            event=("event: error\ndata: "+json.dumps(failure)+"\n\n").encode()
            self.wfile.write(event[:len(event)//2]);self.wfile.flush()
            self.wfile.write(event[len(event)//2:]);self.wfile.flush()
            return
        last = body["messages"][-1]
        has_result = last["role"] == "tool" or any(
            part.get("type") == "tool_result" for part in last.get("content", []) if isinstance(part, dict))
        if has_result:
            if messages_wire:
                result = next(p for p in last["content"] if p["type"] == "tool_result")["content"]
                result = result if isinstance(result, str) else " ".join(p.get("text", "") for p in result)
            else:
                result = text_of(last)
            return self.respond(body, messages_wire, text="WIRE_DONE " + result)
        if "PYTHON_PRIME" in prompt:
            return self.respond(body,messages_wire,tool="python_repl",call={"code":"marker_before_provider = 42\nprint('PYTHON_PRIMED')"})
        if "BLOCK_WIRE" in prompt:
            return self.respond(body,messages_wire,tool="shell",call={"command":"printf 'BLOCK_READY\\n'; sleep 300","timeout_secs":300})
        if "PYTHON_WIRE" in prompt:
            return self.respond(body, messages_wire, tool="python_repl", call={"code":
                "import os\nassert 'VIVY_CHAT_KEY' not in os.environ\nassert 'VIVY_MESSAGES_KEY' not in os.environ\nprint('PYTHON_FILTER_OK')"})
        if "TOOLS_WIRE" in prompt:
            return self.respond(body, messages_wire, tool="shell", call={"command":
                "test -z \"${VIVY_CHAT_KEY+x}\" && test -z \"${VIVY_MESSAGES_KEY+x}\" && printf SHELL_FILTER_OK"})
        return self.respond(body, messages_wire, text="WIRE_TEXT_OK")

    def respond(self, body, messages_wire, text=None, tool=None, call=None):
        if not messages_wire:
            return self.reply(body, text=text, tool=tool or "shell", call=call)
        block = {"type": "text", "text": text} if call is None else {
            "type": "tool_use", "id": "tool-fixture-1", "name": tool, "input": call}
        reason = "end_turn" if call is None else "tool_use"
        message = {"id": "msg-fixture-1", "type": "message", "role": "assistant", "model": body["model"],
                   "content": [block], "stop_reason": reason, "stop_sequence": None,
                   "usage": {"input_tokens": 20, "output_tokens": 10}}
        if not body.get("stream"):
            data = json.dumps(message).encode()
            self.send_response(200)
            self.send_header("content-type", "application/json")
            self.send_header("content-length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        self.send_response(200)
        self.send_header("content-type", "text/event-stream")
        self.end_headers()
        start = dict(message, content=[], stop_reason=None)
        start["usage"] = {"input_tokens": 20, "output_tokens": 0}
        initial = dict(block, **({"text": ""} if call is None else {"input": {}}))
        delta = {"type": "text_delta", "text": text} if call is None else {
            "type": "input_json_delta", "partial_json": json.dumps(call)}
        for event in [
            {"type": "message_start", "message": start},
            {"type": "content_block_start", "index": 0, "content_block": initial},
            {"type": "content_block_delta", "index": 0, "delta": delta},
            {"type": "content_block_stop", "index": 0},
            {"type": "message_delta", "delta": {"stop_reason": reason, "stop_sequence": None}, "usage": {"output_tokens": 10}},
            {"type": "message_stop"},
        ]:
            self.wfile.write(("event: " + event["type"] + "\ndata: " + json.dumps(event) + "\n\n").encode())
            self.wfile.flush()


def main(binary):
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    server.records = []
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="vivy providers ") as scratch:
            root = pathlib.Path(scratch)
            home = root / "home"
            home.mkdir()
            settings = f'''[providers]
default_model = "chat/org/model-1"
[providers.instances.chat]
protocol = "openai"
base_url = "http://127.0.0.1:{server.server_port}/chat/v1"
credential = {{ source = "env", name = "VIVY_CHAT_KEY" }}
[[providers.instances.chat.models]]
id = "org/model-1"
[providers.instances.messages]
protocol = "anthropic"
base_url = "http://127.0.0.1:{server.server_port}/messages"
credential = {{ source = "env", name = "VIVY_MESSAGES_KEY" }}
[[providers.instances.messages.models]]
id = "org/model-1"
max_output_tokens = 1000
'''
            (home / "settings.toml").write_text(settings, encoding="utf-8")
            env = dict(os.environ, RIG_HOME=str(home), VIVY_CHAT_KEY="fixture-chat-private", VIVY_MESSAGES_KEY="fixture-messages-private")

            def run(*args, code=0, environment=None):
                result = subprocess.run([binary, *args], cwd=root, env=environment or env, text=True, encoding="utf-8", capture_output=True, timeout=35)
                assert result.returncode == code, (args, result.returncode, result.stdout, result.stderr)
                assert "fixture-chat-private" not in result.stdout + result.stderr
                assert "fixture-messages-private" not in result.stdout + result.stderr
                assert "fixture-stored-private" not in result.stdout + result.stderr
                assert "fixture-bootstrap-private" not in result.stdout + result.stderr
                assert "refresh_models" not in result.stderr, result.stderr
                return result.stdout + result.stderr

            for provider in ["chat", "messages"]:
                before = {p.name for p in (home / "sessions").iterdir()} if (home / "sessions").exists() else set()
                assert "SHELL_FILTER_OK" in run("--model", f"{provider}/org/model-1", "-p", "TOOLS_WIRE")
                session = next(p.name for p in (home / "sessions").iterdir() if p.name not in before)
                assert "PYTHON_FILTER_OK" in run("--resume", session, "-p", "PYTHON_WIRE")
                assert ("/v1/messages" in server.records[-1][0]) == (provider == "messages")
                assert "WIRE_TEXT_OK" in run("--resume", session, "-p", "NEXT_WIRE")
                run("--model", f"{provider}/org/model-1", "-p", "FAIL_WIRE", code=1)
                run("--model", f"{provider}/org/model-1", "-p", "FAIL_ECHO", code=1)
                run("--model", f"{provider}/org/model-1", "-p", "FAIL_STREAM_ECHO", code=1)
            # The upgraded print front distinguishes invocation errors (2)
            # from a model/tool failure after admission (1).
            run("--model", "gcp.gemini/anything", "-p", "REJECT", code=2)
            assert server.records
            for path, body, headers in server.records:
                assert body["model"] == "org/model-1"
                assert body.get("stream") is True
                if path.endswith("/v1/messages"):
                    assert headers.get("x-api-key") == "fixture-messages-private", headers.keys()
                    assert body.get("max_tokens") == 1000, body.get("max_tokens")
                else:
                    assert path == "/chat/v1/chat/completions", path
                    assert headers.get("authorization") == "Bearer fixture-chat-private", headers.keys()
            keys=home/"auth"/"provider-keys"
            keys.mkdir(parents=True,exist_ok=True)
            (keys/"chat.json").write_text(json.dumps({"key":"fixture-stored-private"}),encoding="utf-8")
            (home/"settings.toml").write_text(settings.replace('credential = { source = "env", name = "VIVY_CHAT_KEY" }','credential = { source = "stored" }'),encoding="utf-8")
            run("--model","chat/org/model-1","-p","FAIL_ECHO",code=1)
            run("--model","chat/org/model-1","-p","FAIL_STREAM_ECHO",code=1)
            assert server.records[-1][2].get("authorization")=="Bearer fixture-stored-private"
            keyless_settings=settings.replace('credential = { source = "env", name = "VIVY_CHAT_KEY" }','credential = { source = "none" }').replace('credential = { source = "env", name = "VIVY_MESSAGES_KEY" }','credential = { source = "none" }')
            (home/"settings.toml").write_text(keyless_settings,encoding="utf-8")
            for provider in ["chat","messages"]:
                assert "WIRE_TEXT_OK" in run("--model",f"{provider}/org/model-1","-p","NEXT_WIRE")
                assert not server.records[-1][2].get("authorization")
                assert not server.records[-1][2].get("x-api-key")
            bootstrap_home=root/"bootstrap"
            bootstrap_env=dict(env,RIG_HOME=str(bootstrap_home),RIG_SECRET_GATEWAY="fixture-bootstrap-private",
                VIVY_BOOTSTRAP_GATEWAY_URL=f"http://127.0.0.1:{server.server_port}/bootstrap/v1",VIVY_BOOTSTRAP_GATEWAY_MODEL="org/bootstrap")
            assert "WIRE_TEXT_OK" in run("-p","NEXT_WIRE",environment=bootstrap_env)
            imported=tomllib.loads((bootstrap_home/"settings.toml").read_text())
            assert imported["providers"]["default_model"]=="gateway/org/bootstrap"
            assert "fixture-bootstrap-private" not in (bootstrap_home/"settings.toml").read_text()
            bootstrap_env["VIVY_BOOTSTRAP_GATEWAY_MODEL"]="org/ignored"
            assert "WIRE_TEXT_OK" in run("-p","NEXT_WIRE",environment=bootstrap_env)
            assert server.records[-1][1]["model"]=="org/bootstrap"
            private_values=[b"fixture-chat-private",b"fixture-messages-private",b"fixture-stored-private",b"fixture-bootstrap-private"]
            for directory in [home,bootstrap_home]:
                for path in directory.rglob("*"):
                    if path.is_file() and keys not in path.parents:
                        data=path.read_bytes()
                        assert not any(key in data for key in private_values),f"credential in persisted record: {path.relative_to(root)}"
            print("PASS: both provider wires, independent credentials, unchanged model IDs, tools, Python filtering, resume and failures")
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main(str(pathlib.Path(sys.argv[1]).resolve()))

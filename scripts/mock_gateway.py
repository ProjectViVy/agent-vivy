"""A scripted OpenAI-compatible Chat Completions server, for offline wiring checks.

It checks how the agent talks to a gateway; it is not a model, and a run against
it is not a real-gateway smoke. Usage, with the agent's key set to `test-key`:

    python3 -I scripts/mock_gateway.py 18080 /tmp/mock.log
    # settings.toml: [gateway] base_url = "http://127.0.0.1:18080/v1", model = "org/model-1"
    RIG_SECRET_GATEWAY=test-key coding-agent --model gateway/org/model-1 -p "create hello.txt"

Logs each request (path, whether a bearer key was sent, model, stream) to the
file given as argv[2]. Replies:
- after a tool result: text quoting the tool output;
- a prompt containing SLEEP: a shell call `sleep 300` (cancellation check);
- a prompt containing FAIL: HTTP 401 (error visibility check);
- a prompt containing SEARCH: a `search` call for "hi";
- otherwise: a shell call that writes hello.txt and counts RIG_SECRET_ vars.
"""
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = 18080
LOG = "mock.log"


def text_of(message):
    content = message.get("content")
    if isinstance(content, list):
        return " ".join(part.get("text", "") for part in content if isinstance(part, dict))
    return content or ""


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers.get("content-length", 0))))
        auth = self.headers.get("authorization", "")
        messages = body.get("messages", [])
        last = messages[-1] if messages else {}
        users = [text_of(m) for m in messages if m.get("role") == "user"]
        prompt = users[-1] if users else ""
        with open(LOG, "a") as log:
            log.write(json.dumps({
                "path": self.path,
                "bearer_test_key": auth == "Bearer test-key",
                "model": body.get("model"),
                "stream": body.get("stream"),
                "last_role": last.get("role"),
                "tools": [t.get("function", {}).get("name") for t in body.get("tools", [])],
                "user_turns": len(users),
                "prompt_start": prompt[:120],
            }) + "\n")
        if not self.path.endswith("/chat/completions"):
            return self.reply_error(404, "unknown path " + self.path)
        if "FAIL" in prompt and last.get("role") == "user":
            return self.reply_error(401, "invalid api key")
        if last.get("role") == "tool":
            return self.reply(body, text="Tool output: " + text_of(last)[:300])
        if "SEARCH" in prompt:
            return self.reply(body, call={"pattern": "hi"}, tool="search")
        if "SLEEP" in prompt:
            return self.reply(body, call={"command": "sleep 300"})
        if users and len(users) > 1:
            return self.reply(body, text="Second turn seen; earlier prompt was: " + users[0][:80])
        return self.reply(body, call={
            "command": "printf hi > hello.txt && cat hello.txt && echo && env | grep -c RIG_SECRET_ || true"
        })

    def reply_error(self, code, message):
        data = json.dumps({"error": {"message": message, "type": "invalid_request_error"}}).encode()
        self.send_response(code)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def reply(self, body, text=None, call=None, tool="shell"):
        model = body.get("model")
        if call is not None:
            delta = {"role": "assistant", "tool_calls": [{
                "index": 0, "id": "call_1", "type": "function",
                "function": {"name": tool, "arguments": json.dumps(call)},
            }]}
            finish = "tool_calls"
        else:
            delta = {"role": "assistant", "content": text}
            finish = "stop"
        # About what the request and reply hold, so compaction sees real sizes.
        prompt_tokens = len(json.dumps(body.get("messages", []))) // 4
        completion_tokens = len(json.dumps(delta)) // 4
        usage = {"prompt_tokens": prompt_tokens, "completion_tokens": completion_tokens,
                 "total_tokens": prompt_tokens + completion_tokens}
        if not body.get("stream"):
            message = {"role": "assistant", "content": delta.get("content")}
            if call is not None:
                message["tool_calls"] = [dict(delta["tool_calls"][0])]
                del message["tool_calls"][0]["index"]
            data = json.dumps({
                "id": "chatcmpl-1", "object": "chat.completion", "created": 0, "model": model,
                "choices": [{"index": 0, "message": message, "finish_reason": finish}],
                "usage": usage,
            }).encode()
            self.send_response(200)
            self.send_header("content-type", "application/json")
            self.send_header("content-length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        self.send_response(200)
        self.send_header("content-type", "text/event-stream")
        self.end_headers()
        chunks = [
            {"choices": [{"index": 0, "delta": delta, "finish_reason": None}]},
            {"choices": [{"index": 0, "delta": {}, "finish_reason": finish}]},
            {"choices": [], "usage": usage},
        ]
        for chunk in chunks:
            chunk.update({"id": "chatcmpl-1", "object": "chat.completion.chunk", "created": 0, "model": model})
            self.wfile.write(b"data: " + json.dumps(chunk).encode() + b"\n\n")
            self.wfile.flush()
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()


if __name__ == "__main__":
    PORT = int(sys.argv[1])
    LOG = sys.argv[2]
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()

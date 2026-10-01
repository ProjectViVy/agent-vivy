#!/usr/bin/env python3
"""Scripted OpenAI-compatible provider for the FFI smoke test.

First request answers with a write_note tool call (which the smoke governance
profile holds for approval); later requests answer with plain text after a
short delay so a concurrent run/cancel has a live run to cancel.
"""
import json
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

TOOL_NAME = "write_note"
NON_TOOL_DELAY_S = 1.0


def tool_call_body(stream: bool):
    tool_call = {
        "index": 0,
        "id": "call_1",
        "type": "function",
        "function": {"name": TOOL_NAME, "arguments": '{"content":"hi"}'},
    }
    if not stream:
        return {
            "id": "chatcmpl-tool",
            "object": "chat.completion",
            "created": 1,
            "model": "deepseek-flash",
            "choices": [{
                "index": 0,
                "message": {"role": "assistant", "content": None, "tool_calls": [tool_call]},
                "finish_reason": "tool_calls",
            }],
            "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
        }
    chunks = []
    base = {"id": "c1", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-flash"}
    t0 = dict(tool_call)
    t0["function"] = dict(tool_call["function"], arguments="")
    chunks.append(dict(base, choices=[{"index": 0, "delta": {"role": "assistant", "tool_calls": [t0]}, "finish_reason": None}]))
    t1 = {"index": 0, "function": {"arguments": '{"content":"hi"}'}}
    chunks.append(dict(base, choices=[{"index": 0, "delta": {"tool_calls": [t1]}, "finish_reason": None}]))
    chunks.append(dict(base, choices=[{"index": 0, "delta": {}, "finish_reason": "tool_calls"}],
                       usage={"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))
    return chunks


def text_body(stream: bool):
    if not stream:
        return {
            "id": "chatcmpl-final",
            "object": "chat.completion",
            "created": 1,
            "model": "deepseek-flash",
            "choices": [{"index": 0, "message": {"role": "assistant", "content": "done"}, "finish_reason": "stop"}],
            "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
        }
    base = {"id": "c2", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-flash"}
    return [
        dict(base, choices=[{"index": 0, "delta": {"role": "assistant", "content": ""}, "finish_reason": None}]),
        dict(base, choices=[{"index": 0, "delta": {"content": "done"}, "finish_reason": None}]),
        dict(base, choices=[{"index": 0, "delta": {}, "finish_reason": "stop"}],
             usage={"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}),
    ]


class Handler(BaseHTTPRequestHandler):
    calls = 0
    lock = threading.Lock()

    def do_POST(self):
        length = int(self.headers.get("Content-Length") or 0)
        body = json.loads(self.rfile.read(length) or b"{}")
        stream = bool(body.get("stream"))
        with Handler.lock:
            Handler.calls += 1
            first = Handler.calls == 1
        if not first:
            time.sleep(NON_TOOL_DELAY_S)
        payload = tool_call_body(stream) if first else text_body(stream)
        if stream:
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            for chunk in payload:
                self.wfile.write(b"data: " + json.dumps(chunk).encode() + b"\n\n")
            self.wfile.write(b"data: [DONE]\n\n")
            self.wfile.flush()
        else:
            raw = json.dumps(payload).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 18321
    ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()

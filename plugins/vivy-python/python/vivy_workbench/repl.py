"""Minimal CPython REPL runtime speaking newline-delimited JSON over stdio.

Entry point: ``python -m vivy_workbench.repl``. Cells execute with top-level await in one persistent
``__main__`` namespace on a single asyncio event loop.
"""

from __future__ import annotations

import ast
import asyncio
import codecs
import contextvars
import ctypes
import functools
import inspect
import io
import json
import linecache
import os
import platform
import select
import signal
import sys
import threading
import time
import traceback
import types
import uuid
from collections.abc import Awaitable, Callable
from typing import Any


PROTOCOL_VERSION = 1

# Stream writes must fit one protocol frame: the host buffers whole lines
# before its per-execution truncation, and raw fd writes already arrive as
# 64 KiB pump chunks.
_STREAM_FRAME_TEXT_CAP = 64 * 1024
# The host truncates results at a smaller per-execution maxChars, so this only
# bounds a pathological repr or exception text in transit.
# JSON can expand one Unicode character to twelve ASCII bytes. An error has
# both evalue and traceback, so two 64 Ki-character fields fit the 2 MiB host
# frame cap even in that worst case. Preserve full text before these caps.
_RESULT_TEXT_CAP = 64 * 1024
_RESULT_TRUNCATION_MARKER = f"\n[... result truncated at {_RESULT_TEXT_CAP} characters ...]"
# Oversized display and host_request payloads fail the cell instead of wedging host memory.
_PAYLOAD_CAP = 1024 * 1024

# Names the session bootstrap re-creates on every start.
_ALWAYS_SKIP = {"workbench", "mcp", "memory", "asyncio", "exit", "quit", "open"}
_protocol_fd: int = -1
# The host's stderr before _setup_fds captures fd 2: the only channel that can
# report a dropped protocol frame without feeding it back into the protocol.
_host_stderr_fd: int = -1
_write_lock = threading.Lock()
_loop: asyncio.AbstractEventLoop | None = None
_serve_task: asyncio.Task[Any] | None = None


class _CellExecution:
    def __init__(self) -> None:
        self.finished = asyncio.Event()
        self.owner: asyncio.Task[Any] | None = None


# Asyncio tasks copy cell context at creation, so detached tasks retain their
# output attribution and completion barrier. Threads start with a fresh context.
_current_cell: contextvars.ContextVar[str | None] = contextvars.ContextVar("_current_cell", default=None)
_current_cell_execution: contextvars.ContextVar[_CellExecution | None] = contextvars.ContextVar(
    "_current_cell_execution", default=None
)
_active: dict[str, Any] = {"task": None, "rid": None, "interrupted": False}
_cell_counter = 0
_pending_host: dict[str, "asyncio.Future[dict[str, Any]]"] = {}
# Set on the loop thread once stdin hits EOF or a shutdown request arrives; no
# host reply can arrive after that, so waiting (and future) host_request calls fail.
_host_closed = False

# In-flight dedicated-lane MCP status requests (the eager settle and the
# connections view ride this lane): the serve loop hands them off instead of
# awaiting them inline, so a hanging server bounds its own request while the
# execution queue stays free for the user's cells. Shutdown cancels the set
# before closing the MCP registry.
_mcp_status_tasks: "set[asyncio.Task[None]]" = set()

# Interrupt bookkeeping shared between the reader thread and the loop thread.
_interrupt_lock = threading.Lock()
_inflight: set[str] = set()
_pending_interrupts: dict[str, Any] = {"ids": set(), "any": False}
_sigint_target: str | None = None
_finishing_rid: str | None = None
_handoff_interrupted = False


def _send(event: dict[str, Any]) -> None:
    """Write one protocol frame; the locked single write keeps frames atomic."""
    data = (json.dumps(event, separators=(",", ":")) + "\n").encode()
    with _write_lock:
        view = memoryview(data)
        try:
            while view:
                view = view[os.write(_protocol_fd, view) :]
        except OSError as err:
            try:
                os.write(_host_stderr_fd, f"vivy_workbench.repl: dropped protocol frame: {err}\n".encode())
            except OSError:
                pass


def _check_payload(event: str, data: dict[str, Any]) -> None:
    """Fail the calling cell when a `data` payload would not fit one protocol frame.

    Strict-dumps validation: default allow_nan=True would let NaN/Infinity
    serialize as non-JSON text and tear the host's protocol framing (a
    non-serializable value raises TypeError here before any bytes are
    written, so NaN is the only corruption vector). The encoded length
    enforces the frame cap; _send re-serializes.
    """
    if len(json.dumps(data, allow_nan=False)) > _PAYLOAD_CAP:
        raise ValueError(f"{event} payload exceeds the {_PAYLOAD_CAP}-character frame cap")


def emit(data: dict[str, Any]) -> None:
    """Ship one display event carrying a dict of MIME type -> JSON payload.

    Thread-safe; the event is tagged with the cell running at call time.
    """
    if not isinstance(data, dict) or not data or not all(isinstance(k, str) for k in data):
        raise TypeError("emit() requires a non-empty dict keyed by MIME type strings")
    from .artifacts import save
    artifact = save(data, 'application/json')
    _send({"event": "display", "id": _current_cell.get(), "artifact": artifact})


def is_active() -> bool:
    """True when this process serves the repl protocol (not merely imported)."""
    return _protocol_fd >= 0


def current_cell_completion_context() -> tuple[asyncio.Event, asyncio.Task[Any] | None] | None:
    """Return the calling cell's completion barrier and owning execution task."""
    execution = _current_cell_execution.get()
    if execution is None:
        return None
    return execution.finished, execution.owner


def active_cell_task() -> asyncio.Task[Any] | None:
    """The cell body task executing right now, or None between cells (global
    state, not the cell contextvar — detached tasks keep stale context copies)."""
    with _interrupt_lock:
        task = _active["task"]
    return task if isinstance(task, asyncio.Task) and not task.done() else None


async def host_request(data: dict[str, Any]) -> dict[str, Any]:
    """Send one typed request to the host and await its raw reply dict."""
    if _loop is None:
        raise RuntimeError("repl runtime is not serving")
    if _host_closed:
        raise RuntimeError("host connection closed; host_request cannot be answered")
    _check_payload("host_request", data)
    rid = uuid.uuid4().hex
    future: asyncio.Future[dict[str, Any]] = _loop.create_future()
    _pending_host[rid] = future
    try:
        _send({"event": "host_request", "id": rid, "data": data})
        return await future
    finally:
        _pending_host.pop(rid, None)


def _fail_pending_host_requests() -> None:
    """Loop-thread half of teardown: no host reply can arrive anymore, so every
    awaiting cell must unblock or the queued shutdown would never be served."""
    global _host_closed
    _host_closed = True
    for future in _pending_host.values():
        if not future.done():
            future.set_exception(RuntimeError("host connection closed; host_request cannot be answered"))


def _resolve_host_reply(rid: str, data: dict[str, Any]) -> None:
    """Reader-thread half of the host bridge; late/unknown replies are dropped."""
    assert _loop is not None

    def deliver() -> None:
        future = _pending_host.get(rid)
        if future is not None and not future.done():
            future.set_result(data)

    _loop.call_soon_threadsafe(deliver)


class _Pump:
    """Reads one captured-output pipe and ships its bytes as stream events."""

    def __init__(self, read_fd: int, write_fd: int, stream: str) -> None:
        self._read_fd = read_fd
        # Private write end: a cell closing/reclaiming fd 1/2 cannot hijack drain tokens.
        self._token_fd = os.dup(write_fd)
        self._stream = stream
        self._decoder = codecs.getincrementaldecoder("utf-8")("replace")
        self._lock = threading.Lock()
        self._watch: tuple[bytes, threading.Event] | None = None
        self._buf = b""
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()

    def drain(self) -> None:
        """Block until every byte written to the fd so far has been shipped."""
        token = b"\xff<drain:" + uuid.uuid4().hex.encode() + b">\xff"
        seen = threading.Event()
        with self._lock:
            self._watch = (token, seen)
        try:
            os.write(self._token_fd, token)
            # Backstop only: a dead pump (read end closed under it) can never set seen.
            while not seen.wait(0.1):
                if not self._thread.is_alive():
                    return
        except OSError:
            return
        finally:
            with self._lock:
                self._watch = None

    def _run(self) -> None:
        while True:
            try:
                chunk = os.read(self._read_fd, 65536)
            except OSError:
                break
            if not chunk:
                break
            self._feed(chunk)

    def _feed(self, chunk: bytes) -> None:
        data = self._buf + chunk
        self._buf = b""
        with self._lock:
            watch = self._watch
        if watch is None:
            self._emit(data)
            return
        token, seen = watch
        while True:
            i = data.find(token)
            if i == -1:
                break
            self._emit(data[:i])
            self._finish_decode()
            seen.set()
            data = data[i + len(token) :]
        # Hold back a tail that could be the start of a token split across reads.
        hold = 0
        for k in range(min(len(data), len(token) - 1), 0, -1):
            if data.endswith(token[:k]):
                hold = k
                break
        if hold:
            self._buf = data[len(data) - hold :]
            data = data[: len(data) - hold]
        self._emit(data)

    def _emit(self, data: bytes) -> None:
        if not data:
            return
        text = self._decoder.decode(data)
        if text:
            # Raw fd bytes have no provable owner (os.write, C extensions,
            # subprocesses, threads from earlier cells): never credit a cell.
            _send({"event": self._stream, "id": None, "text": text})

    def _finish_decode(self) -> None:
        text = self._decoder.decode(b"", final=True)
        self._decoder = codecs.getincrementaldecoder("utf-8")("replace")
        if text:
            _send({"event": self._stream, "id": None, "text": text})


class _TaggedBuffer(io.RawIOBase):
    """Binary proxy for _TaggedWriter.buffer: bytes go to the raw fd channel.

    Byte ownership cannot be proven at this layer, so buffer writes ride the
    captured pipe and surface as id:null stream events (drained before done
    like any other raw fd write).
    """

    def __init__(self, fallback_fd: int) -> None:
        self._fallback_fd = fallback_fd

    def write(self, data: Any) -> int:
        # memoryview (unlike bytes()) rejects int, matching a real buffer's TypeError.
        view = memoryview(data).cast("B")
        total = len(view)
        # Pipe writes can be short for payloads above the pipe capacity.
        while view:
            view = view[os.write(self._fallback_fd, view) :]
        return total

    def flush(self) -> None:
        pass

    def fileno(self) -> int:
        return self._fallback_fd

    def writable(self) -> bool:
        return True


class _TaggedWriter(io.TextIOBase):
    """sys.stdout/sys.stderr replacement tagging writes with the writer's cell id.

    Python-level writes carry write-time provenance from the _current_cell
    contextvar (asyncio tasks inherit the spawning cell's id; user threads see
    None) and ship straight to the protocol, bypassing the fd pipe. fileno()
    and .buffer expose the captured pipe so subprocesses, C-level writers, and
    sys.stdout.buffer.write() keep working through the raw channel
    (null-attributed).
    """

    def __init__(self, stream: str, fallback_fd: int) -> None:
        self._stream = stream
        self._fallback_fd = fallback_fd
        # Keeps one write()'s frames contiguous under concurrent writers.
        self._frame_lock = threading.Lock()
        self._buffer = _TaggedBuffer(fallback_fd)

    def write(self, text: str) -> int:
        if not isinstance(text, str):
            raise TypeError(f"write() argument must be str, not {type(text).__name__}")
        if text:
            cell_id = _current_cell.get()
            with self._frame_lock:
                for start in range(0, len(text), _STREAM_FRAME_TEXT_CAP):
                    _send(
                        {
                            "event": self._stream,
                            "id": cell_id,
                            "text": text[start : start + _STREAM_FRAME_TEXT_CAP],
                        }
                    )
        return len(text)

    def flush(self) -> None:
        pass

    def fileno(self) -> int:
        return self._fallback_fd

    def writable(self) -> bool:
        return True

    @property
    def buffer(self) -> _TaggedBuffer:
        return self._buffer

    @property
    def encoding(self) -> str:
        return "utf-8"

    @property
    def errors(self) -> str:
        return "replace"


def _consume_task_exception(task: asyncio.Task[Any]) -> None:
    """Retrieve a killed task's exception so no never-retrieved noise is logged."""
    if not task.cancelled():
        task.exception()


def _sigint_handler(signum: int, frame: types.FrameType | None) -> None:
    global _handoff_interrupted
    task = _active["task"]
    # No lock (the main thread may hold it): the rid equality revalidates the
    # target so a SIGINT delayed past its request's finish cannot hit a later cell.
    if task is None or task.done() or _active["rid"] != _sigint_target:
        if _sigint_target is not None and _sigint_target == _active["rid"]:
            # Handoff: the task is done but _run_guarded's finally has not run
            # yet, so the main thread may be inside loop internals where raising
            # would kill the serve loop. Record it; the finishing phase consumes it.
            _handoff_interrupted = True
            return
        # Post-run repr/drain is synchronous main-thread work: raise into it.
        # The equality revalidation drops a SIGINT delayed past the done send.
        if _sigint_target is not None and _sigint_target == _finishing_rid:
            raise KeyboardInterrupt
        return
    _active["interrupted"] = True
    # Handler runs in the main (loop) thread: current_task is whose step the signal interrupted.
    running = asyncio.current_task(_loop) if _loop is not None else None
    if running is task:
        # The active request's own step (sync bytecode or an EINTR-woken syscall): raise into it.
        raise KeyboardInterrupt
    # Loop idle in select() or another task mid-step: cancel the active task (same thread, safe).
    task.cancel()
    if running is not None and running is not _serve_task:
        # A background task blocked in sync code occupies the only thread and would keep the
        # cancel from ever running: raise into it to unwind its step; it dies with the KI.
        running.add_done_callback(_consume_task_exception)
        raise KeyboardInterrupt


def _request_interrupt(target: str | None) -> None:
    """Deliver an interrupt now, or park it for the request it targets.

    Runs on the reader thread. Without a target id the interrupt applies to
    the running request, else to the next queued one; with a target id it
    applies to that request only. A request finishing its post-run repr/drain
    is still interrupted (never parked: parking would hit the NEXT request).
    Interrupts for finished or unknown requests are dropped.
    """
    global _sigint_target
    with _interrupt_lock:
        task = _active["task"]
        rid = _active["rid"]
        if rid is not None and (target is None or target == rid):
            # Active, or in the done-task handoff before _run_guarded's finally:
            # either way the rid still owns the interrupt (parking here would
            # leak it onto the next request); the handler decides delivery.
            _sigint_target = rid
        elif _finishing_rid is not None and (target is None or target == _finishing_rid):
            _sigint_target = _finishing_rid
        elif target is not None:
            if target in _inflight:
                _pending_interrupts["ids"].add(target)
            return
        elif _inflight:
            _pending_interrupts["any"] = True
            return
        else:
            return
    # SIGINT must land on the main thread, where cells execute. Windows has no
    # signal.pthread_kill: fall back to cancelling the active task on the loop
    # (sync-blocked cells and the finishing repr/drain cannot be broken there;
    # best-effort parity).
    if hasattr(signal, "pthread_kill"):
        signal.pthread_kill(threading.main_thread().ident, signal.SIGINT)
        if _loop is not None:
            # Wake the selector so a cancel scheduled by the handler runs promptly.
            _loop.call_soon_threadsafe(lambda: None)
        return
    if _loop is not None:

        def cancel_active() -> None:
            current = _active["task"]
            if current is task and current is not None and not current.done():
                _active["interrupted"] = True
                current.cancel()

        _loop.call_soon_threadsafe(cancel_active)


def _consume_pending_interrupt(rid: str) -> bool:
    """Check-and-clear any interrupt parked for this request."""
    pending = _pending_interrupts["any"] or rid in _pending_interrupts["ids"]
    _pending_interrupts["any"] = False
    _pending_interrupts["ids"].discard(rid)
    return pending


def _consume_handoff_interrupt() -> bool:
    """Check-and-clear an interrupt that landed in the done-task handoff."""
    global _handoff_interrupted
    with _interrupt_lock:
        pending = _handoff_interrupted
        _handoff_interrupted = False
        return pending


def _finish_locked(rid: str) -> None:
    """Drop a finished request; a parked untargeted interrupt survives while others are inflight."""
    global _finishing_rid, _handoff_interrupted, _sigint_target
    if _finishing_rid == rid:
        # An unconsumed handoff interrupt dies with its request (state requests
        # have no cancellable post-run work); it must never hit the next request.
        _finishing_rid = None
        _handoff_interrupted = False
    if _sigint_target == rid:
        # The target dies with its request: a later request reusing this id must
        # not match a stale target when a delayed/external SIGINT arrives.
        _sigint_target = None
    _inflight.discard(rid)
    _pending_interrupts["ids"].discard(rid)
    if not _inflight:
        _pending_interrupts["any"] = False


def _finish_request(rid: str) -> None:
    with _interrupt_lock:
        _finish_locked(rid)


_RUNTIME_FILE = __file__


def _cell_stack(stack: traceback.StackSummary) -> traceback.StackSummary | None:
    """Frames from the first cell frame on, minus runtime-internal frames.

    Returns None when no cell frame exists (e.g. a compile-time SyntaxError).
    """
    start = next((i for i, f in enumerate(stack) if f.filename.startswith("<cell-")), None)
    if start is None:
        return None
    return traceback.StackSummary.from_list([f for f in stack[start:] if f.filename != _RUNTIME_FILE])


def _safe_str(exc: BaseException) -> str:
    try:
        return str(exc)
    except BaseException:  # noqa: BLE001 - a broken __str__ must not kill the runtime
        return "<exception str() failed>"


def _cap_text(text: str) -> str:
    if len(text) > _RESULT_TEXT_CAP:
        return text[:_RESULT_TEXT_CAP] + _RESULT_TRUNCATION_MARKER
    return text


def _cap_traceback_lines(lines: list[str]) -> list[str]:
    """Bound the aggregate, not just each entry: an exception chain can carry
    thousands of entries, and per-entry caps alone would still let one error
    event exceed the host's protocol line limit. Keep the newest entries — the
    outermost exception carries the actionable failure — and lead with a
    truncation marker."""
    total = sum(len(line) for line in lines)
    if total <= _RESULT_TEXT_CAP:
        return lines
    kept: list[str] = []
    remaining = _RESULT_TEXT_CAP
    for line in reversed(lines):
        if len(line) > remaining:
            if not kept:
                # Never drop the newest entry: it names the raised exception.
                kept.append(line)
            break
        kept.append(line)
        remaining -= len(line)
    kept.reverse()
    kept.insert(
        0,
        f"[... traceback truncated: kept the newest {len(kept)} of {len(lines)} entries "
        f"to fit {_RESULT_TEXT_CAP} characters ...]\n",
    )
    return kept


def _error_event(cell_id: str, exc: BaseException) -> dict[str, Any]:
    # No cell frame (e.g. SyntaxError): exception-only keeps filename, source, and caret.
    te = traceback.TracebackException.from_exception(exc)
    stack = _cell_stack(te.stack)
    if stack is None:
        lines = traceback.format_exception_only(type(exc), exc)
    else:
        te.stack = stack
        lines = list(te.format())
    full = ''.join(lines)
    if len(full) > _RESULT_TEXT_CAP:
        from .artifacts import save
        artifact = save(full, 'text/plain')
        _send({'event': 'artifact', 'id': cell_id, 'artifact': artifact})
    return {
        "event": "error",
        "id": cell_id,
        "ename": type(exc).__name__[:512],
        "evalue": _cap_text(_safe_str(exc)),
        "traceback": _cap_traceback_lines([_cap_text(line) for line in lines]),
    }


def _interrupt_event(cell_id: str, exc: BaseException) -> dict[str, Any]:
    """Report a cancelled await-suspended cell as a KeyboardInterrupt."""
    stack = _cell_stack(traceback.extract_tb(exc.__traceback__))
    lines = []
    if stack:
        lines = ["Traceback (most recent call last):\n"]
        lines.extend(stack.format())
    lines.append("KeyboardInterrupt\n")
    return {"event": "error", "id": cell_id, "ename": "KeyboardInterrupt", "evalue": "", "traceback": lines}


def _compile_cell(code: str, filename: str) -> tuple[list[types.CodeType], bool]:
    """Compile a cell; a trailing expression compiles separately in eval mode."""
    linecache.cache[filename] = (len(code), None, code.splitlines(keepends=True), filename)
    tree = ast.parse(code, filename)
    trailing: ast.Expression | None = None
    if tree.body and isinstance(tree.body[-1], ast.Expr):
        trailing = ast.Expression(tree.body.pop().value)
    flags = ast.PyCF_ALLOW_TOP_LEVEL_AWAIT
    codes: list[types.CodeType] = []
    if tree.body:
        codes.append(compile(tree, filename, "exec", flags=flags, dont_inherit=True))
    if trailing is not None:
        codes.append(compile(trailing, filename, "eval", flags=flags, dont_inherit=True))
    return codes, trailing is not None


async def _run_codes(codes: list[types.CodeType], ns: dict[str, Any]) -> Any:
    value: Any = None
    for code_obj in codes:
        value = eval(code_obj, ns)  # noqa: S307 - executing the model's cell is the runtime's job
        if code_obj.co_flags & inspect.CO_COROUTINE:
            value = await value
    return value


async def _run_guarded(task: asyncio.Task[Any], rid: str) -> tuple[str, Any, dict[str, Any] | None]:
    """Await a request task; returns (status, value, error event or None)."""
    with _interrupt_lock:
        _active["interrupted"] = False
        _active["rid"] = rid
        _active["task"] = task
        if _consume_pending_interrupt(rid):
            # Interrupt parked before activation: cancel before the first step.
            _active["interrupted"] = True
            task.cancel()
    try:
        value = await task
        return "ok", value, None
    except asyncio.CancelledError as exc:
        if _active["interrupted"]:
            return "error", None, _interrupt_event(rid, exc)
        return "error", None, _error_event(rid, exc)
    except BaseException as exc:  # noqa: BLE001 - every cell failure becomes an error event
        return "error", None, _error_event(rid, exc)
    finally:
        with _interrupt_lock:
            global _finishing_rid
            # The rid stays inflight and interrupt-targetable through the
            # post-run repr/drain; the handler closes the window via _finish_request.
            # Set before clearing _active: the lock-free handler must always see
            # the rid in one of the two slots, never a torn in-between state.
            _finishing_rid = rid
            _active["task"] = None
            _active["rid"] = None


async def _memory_call(req: dict[str, Any], ns: dict[str, Any]) -> Any:
    from .memory import Memory
    operations = {'remember': Memory.remember, 'get': Memory.get, 'search': Memory.search}
    action, arguments = req['action'], req.get('arguments')
    if action not in operations or not isinstance(arguments, dict):
        raise ValueError('memory requires remember/get/search and an arguments object')
    memory = ns.get('memory')
    if type(memory) is not Memory:
        raise RuntimeError('Native memory namespace is unavailable; enable and prepare memory, then reset the worker')
    # Fixed facade methods, never eval or a model-selected Python attribute.
    return await operations[action](memory, **arguments)


async def _handle_memory(req: dict[str, Any], ns: dict[str, Any]) -> None:
    rid = req['id']
    token = _current_cell.set(rid)
    try:
        status, value, error = await _run_guarded(asyncio.create_task(_memory_call(req, ns)), rid)
        text = None
        try:
            if _consume_handoff_interrupt() and status == 'ok':
                status, error = 'error', _error_event(rid, KeyboardInterrupt())
            if status == 'ok':
                text = json.dumps(value, ensure_ascii=False)
                if len(text) > _RESULT_TEXT_CAP:
                    from .artifacts import save
                    artifact = save(text, 'application/json')
                    _send({'event': 'artifact', 'id': rid, 'artifact': artifact})
                text = _cap_text(text)
            _drain_output()
        finally:
            _finish_request(rid)
        if text is not None:
            _send({'event': 'result', 'id': rid, 'text': text})
        if error is not None:
            _send(error)
        from .capabilities import memory_status
        _send({'event': 'done', 'id': rid, 'status': status, 'memory': memory_status(ns)})
    finally:
        _current_cell.reset(token)


async def _handle_execute(req: dict[str, Any], ns: dict[str, Any]) -> None:
    global _cell_counter
    cell_id = req["id"]
    _cell_counter += 1
    filename = f"<cell-{_cell_counter}>"
    execution = _CellExecution()
    cell_token = _current_cell.set(cell_id)
    execution_token = _current_cell_execution.set(execution)
    try:
        codes, has_trailing = _compile_cell(req["code"], filename)
        assert _loop is not None
        task = _loop.create_task(_run_codes(codes, ns))
        execution.owner = task
        status, value, error = await _run_guarded(task, cell_id)
        result_text: str | None = None
        try:
            if _consume_handoff_interrupt() and status == "ok":
                # SIGINT landed between the task's completion and the finishing
                # phase: it targeted this request, so cancel its remaining work.
                status, error = "error", _error_event(cell_id, KeyboardInterrupt())
            if status == "ok" and has_trailing and value is not None:
                try:
                    ns["_"] = value
                    result_text = repr(value)
                except BaseException as exc:  # noqa: BLE001 - a broken __repr__ is a cell error
                    status, error = "error", _error_event(cell_id, exc)
            if result_text is not None:
                if len(result_text) > _RESULT_TEXT_CAP:
                    from .artifacts import save
                    artifact = save(result_text)
                    _send({"event": "artifact", "id": cell_id, "artifact": artifact})
                result_text = _cap_text(result_text)
            _drain_output()
        finally:
            # Close the interrupt window before the protocol sends so a
            # handler-raised KeyboardInterrupt can never tear a frame mid-_send.
            _finish_request(cell_id)
        if result_text is not None:
            _send({"event": "result", "id": cell_id, "text": result_text})
        if error is not None:
            _send(error)
        from .capabilities import memory_status
        _send({"event": "done", "id": cell_id, "status": status, "memory": memory_status(ns)})
    finally:
        execution.owner = None
        execution.finished.set()
        _current_cell_execution.reset(execution_token)
        _current_cell.reset(cell_token)


def _drain_output() -> None:
    # Per-stream, and ValueError too: a cell may close sys.stdout/sys.stderr, and
    # flushing a closed file raises ValueError, or rebind them to a flush-less
    # object (AttributeError); neither may kill the serve loop nor skip flushing
    # the other stream.
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.flush()
        except (OSError, ValueError, AttributeError):
            pass
    _pump_out.drain()
    _pump_err.drain()


def _list_names(ns: dict[str, Any]) -> list[str]:
    """User-defined top-level names, excluding bootstrap bindings."""
    # Non-string keys (globals()[1] = 1) are not user-listable names.
    return sorted(
        name for name in ns if isinstance(name, str) and not name.startswith("_") and name not in _ALWAYS_SKIP
    )


async def _handle_list_names(req: dict[str, Any], ns: dict[str, Any]) -> None:
    _send({"event": "done", "id": req["id"], "status": "ok", "names": _list_names(ns)})


async def _handle_mcp_status(req: dict[str, Any], ns: dict[str, Any]) -> None:
    from . import mcp as mcp_mod

    servers = req.get("servers")
    if not isinstance(servers, list) or not all(isinstance(name, str) for name in servers):
        raise ValueError("mcp_status requires a list of server names")
    timeout_ms = req.get("timeout_ms", 10_000)
    if isinstance(timeout_ms, bool) or not isinstance(timeout_ms, (int, float)) or timeout_ms <= 0:
        raise ValueError("mcp_status timeout_ms must be a positive number")
    connections = await mcp_mod.status(servers, float(timeout_ms))
    _send({"event": "done", "id": req["id"], "status": "ok", "connections": connections})


async def _handle_request(
    handler: Callable[[dict[str, Any], dict[str, Any]], Awaitable[None]],
    req: dict[str, Any],
    ns: dict[str, Any],
) -> None:
    # Backstop: one broken request (e.g. RecursionError in compile) fails alone, never the serve loop.
    try:
        await handler(req, ns)
    except BaseException as exc:  # noqa: BLE001 - any per-request failure becomes error+done
        rid = req["id"]
        with _interrupt_lock:
            # Only a still-inflight request (never reached _run_guarded, e.g. compile
            # failure) owns a parked interrupt; after _run_guarded finished it, a parked
            # "any" belongs to the next request and must survive.
            if rid in _inflight:
                _consume_pending_interrupt(rid)
            _finish_locked(rid)
        _send(_error_event(rid, exc))
        _send({"event": "done", "id": rid, "status": "error"})


async def _mcp_status_lane(req: dict[str, Any], ns: dict[str, Any]) -> None:
    """One dedicated-lane `mcp_status` request.

    The failure shape mirrors `_handle_request`'s backstop (one broken
    request fails alone, never the lane, never the serve loop); a
    shutdown cancellation is silent by design — the host is already
    tearing the kernel down and owns the deadline.
    """
    try:
        await _handle_mcp_status(req, ns)
    except asyncio.CancelledError:
        raise
    except BaseException as exc:  # noqa: BLE001 - one broken lane request fails alone
        rid = req["id"]
        try:
            _send(_error_event(rid, exc))
            _send({"event": "done", "id": rid, "status": "error"})
        except BaseException:
            pass


def _spawn_mcp_status_lane(req: dict[str, Any], ns: dict[str, Any]) -> None:
    """Hand one `mcp_status` request to the dedicated lane.

    `mcp_status` opens and lists the configured generic servers (the eager
    background settle and the connections view both ride it), and a slow or
    unreachable server can hold it for the whole per-server timeout. Awaiting
    it inline on the execution queue would park the user's first python cell
    behind a server handshake that no cell asked for (finding: the eager
    settle must never contend with the user's first-turn path; the join point
    stays at first use through the registry's per-server locks). The lane task
    runs on the same loop (the registry dispatch joins concurrent listings
    and an early user call onto the same in-flight open), so ordering between
    cells is untouched — only status work stops occupying the cell queue.
    """
    task = asyncio.get_running_loop().create_task(_mcp_status_lane(req, ns))
    _mcp_status_tasks.add(task)
    task.add_done_callback(_mcp_status_tasks.discard)


async def _serve(queue: asyncio.Queue[dict[str, Any]], ns: dict[str, Any]) -> None:
    while True:
        req = await queue.get()
        # A cell may have rebound SIGINT; the
        # protocol handler must own it before each request. Mid-cell rebinds remain
        # that cell's own problem for that cell only.
        signal.signal(signal.SIGINT, _sigint_handler)
        rtype = req.get("type")
        if rtype == "shutdown":
            rid = req.get("id")
            # The dedicated MCP status lane goes first: a still-listing
            # server is cancelled (bounded, silent) so the registry close
            # below cannot race an in-flight open under the host's 5s
            # shutdown deadline.
            if _mcp_status_tasks:
                for task in list(_mcp_status_tasks):
                    task.cancel()
                await asyncio.wait(set(_mcp_status_tasks), timeout=1.0)
                _mcp_status_tasks.clear()
            # MCP children must close before the loop dies; close() is internally bounded under the host's 5s deadline.
            mcp_mod = sys.modules.get("vivy_workbench.mcp")
            if mcp_mod is not None:
                try:
                    await mcp_mod.close()
                except BaseException as exc:
                    print(f"MCP shutdown failed: {type(exc).__name__}: {exc}", file=sys.stderr)
            memory = ns.get('memory')
            if memory is not None:
                await memory.close()
            if isinstance(rid, str):
                _send({"event": "done", "id": rid, "status": "ok"})
            return
        if rtype == "execute":
            await _handle_request(_handle_execute, req, ns)
        elif rtype == "memory":
            await _handle_request(_handle_memory, req, ns)
        elif rtype == "list_names":
            await _handle_request(_handle_list_names, req, ns)
        elif rtype == "mcp_status":
            # The dedicated lane (see `_spawn_mcp_status_lane`): status/open
            # work never occupies the execution queue.
            _spawn_mcp_status_lane(req, ns)


_REQUIRED_FIELDS = {
    "execute": ("id", "code"),
    "memory": ("id", "action"),
    "list_names": ("id",),
    # mcp_status's server list is a JSON array, so only its id is a
    # string-required field; the handler validates the list itself.
    "mcp_status": ("id",),
    "shutdown": (),
}


def _protocol_error(message: str) -> None:
    _send({"event": "error", "id": None, "ename": "ProtocolError", "evalue": message, "traceback": []})


def _handle_request_line(raw: bytes, queue: asyncio.Queue[dict[str, Any]]) -> None:
    assert _loop is not None
    req = json.loads(raw)
    if not isinstance(req, dict):
        raise ValueError("request is not a JSON object")
    rtype = req.get("type")
    if rtype == "interrupt":
        if "id" in req and not isinstance(req["id"], str):
            _protocol_error("interrupt request id must be a string")
            return
        _request_interrupt(req.get("id"))
        return
    if rtype == "host_reply":
        # Bypass the FIFO queue: the awaiting cell IS the in-flight
        # execute, so a queued reply would deadlock behind it.
        rid = req.get("id")
        data = req.get("data")
        if isinstance(rid, str) and isinstance(data, dict):
            _resolve_host_reply(rid, data)
        else:
            _protocol_error("host_reply request needs string id and dict data")
        return
    if not isinstance(rtype, str) or rtype not in _REQUIRED_FIELDS:
        _protocol_error(f"unknown request type: {rtype!r}")
        return
    missing = [f for f in _REQUIRED_FIELDS[rtype] if not isinstance(req.get(f), str)]
    if missing:
        _protocol_error(f"{rtype} request needs string fields: {', '.join(missing)}")
        return
    if rtype == "execute":
        with _interrupt_lock:
            # A reused in-flight id would corrupt interrupt/finish bookkeeping.
            duplicate = req["id"] in _inflight
            if not duplicate:
                _inflight.add(req["id"])
        # The protocol write can block on backpressure: never send under the lock.
        if duplicate:
            _protocol_error(f"duplicate in-flight request id: {req['id']!r}")
            return
    if rtype == "shutdown":
        # No host reply follows a shutdown; a cell awaiting host_request
        # must fail now or it would block _serve from ever consuming this.
        _loop.call_soon_threadsafe(_fail_pending_host_requests)
    _loop.call_soon_threadsafe(queue.put_nowait, req)


def _read_requests(stdin_fd: int, queue: asyncio.Queue[dict[str, Any]]) -> None:
    assert _loop is not None
    with os.fdopen(stdin_fd, "rb") as stream:
        while raw := stream.readline(2 * 1024 * 1024 + 1):
            if len(raw) > 2 * 1024 * 1024:
                _protocol_error("request exceeds 2 MiB")
                break
            raw = raw.strip()
            if not raw:
                continue
            try:
                # The whole per-line handling sits inside the backstop: hostile
                # input (RecursionError from pathological nesting, unhashable
                # field types, ...) must never kill the reader thread.
                _handle_request_line(raw, queue)
            except BaseException as err:  # noqa: BLE001
                _protocol_error(f"{type(err).__name__}: {_safe_str(err)}")
    # Host closed stdin: shut the runtime down. The marker distinguishes
    # this from the host's explicit shutdown request (which runs after the
    # host flushed its own final snapshot, so no runtime-side flush runs).
    _loop.call_soon_threadsafe(_fail_pending_host_requests)
    _loop.call_soon_threadsafe(queue.put_nowait, {"type": "shutdown", "eof": True})


def _resolve_owner_pid() -> int:
    raw = os.environ.get("VIVY_WORKBENCH_OWNER_PID", "")
    try:
        owner = int(raw)
    except ValueError:
        owner = 0
    return owner if owner > 0 else os.getppid()


def _owner_alive_posix(owner: int, initial_ppid: int) -> bool:
    # Reparenting is the race-free parent-death signal when the owner is the
    # parent; the kill-0 probe covers an env-designated non-parent owner.
    if initial_ppid == owner and os.getppid() != initial_ppid:
        return False
    try:
        os.kill(owner, 0)
    except ProcessLookupError:
        return False
    except OSError:
        pass  # EPERM etc.: alive but unprobeable
    return True


def _wait_owner_posix(owner: int, initial_ppid: int) -> None:
    # Blocks until the owner exits: the OS exit notification (kqueue NOTE_EXIT
    # on macOS/BSD, a pidfd on Linux) wakes this thread once, with no polling.
    # Registration binds whichever process holds the pid right now, so the
    # liveness check after it catches a parent owner that died (and whose pid
    # may be reused) before the watch existed.
    try:
        if hasattr(select, "kqueue"):
            kq = select.kqueue()
            # max_events=0 makes a registration error raise (ESRCH for a gone
            # owner) instead of arriving as an EV_ERROR event.
            kq.control(
                [select.kevent(owner, select.KQ_FILTER_PROC, select.KQ_EV_ADD, select.KQ_NOTE_EXIT)],
                0,
                0,
            )
            wait_for_exit = functools.partial(kq.control, None, 1)
        else:
            poller = select.poll()
            poller.register(os.pidfd_open(owner), select.POLLIN)
            wait_for_exit = poller.poll
    except ProcessLookupError:
        return  # already gone
    except (AttributeError, OSError):
        # Slow fallback, only where exit notification is unavailable: Linux
        # before 5.3, a seccomp filter denying pidfd_open, a CPython built
        # without os.pidfd_open.
        while _owner_alive_posix(owner, initial_ppid):
            time.sleep(30.0)
        return
    if _owner_alive_posix(owner, initial_ppid):
        wait_for_exit()


def _wait_owner_windows(owner: int) -> None:
    # Blocks until the owner exits. os.kill(pid, 0) on Windows TERMINATES the
    # target, so a SYNCHRONIZE handle wait is the only sound probe.
    from ctypes import wintypes

    SYNCHRONIZE = 0x00100000
    INFINITE = 0xFFFFFFFF
    k32 = ctypes.WinDLL("kernel32", use_last_error=True)
    k32.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
    k32.OpenProcess.restype = wintypes.HANDLE
    k32.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
    k32.WaitForSingleObject.restype = wintypes.DWORD
    k32.CloseHandle.argtypes = [wintypes.HANDLE]
    k32.CloseHandle.restype = wintypes.BOOL
    handle = k32.OpenProcess(SYNCHRONIZE, False, owner)
    if not handle:
        return  # already gone (or unprobeable): exit rather than run ownerless
    try:
        k32.WaitForSingleObject(handle, INFINITE)
    finally:
        k32.CloseHandle(handle)


def _owner_watchdog(owner: int, initial_ppid: int) -> None:
    if os.name == "nt":
        _wait_owner_windows(owner)
    else:
        _wait_owner_posix(owner, initial_ppid)
    # Event-loop-independent by design: a synchronous cell monopolizes the
    # loop, so the queued EOF shutdown can never run; hard-exit from here.
    os._exit(1)


def _start_owner_watchdog() -> None:
    threading.Thread(
        target=_owner_watchdog, args=(_resolve_owner_pid(), os.getppid()), daemon=True
    ).start()


_pump_out: _Pump
_pump_err: _Pump


def _replace_fd(fd: int, source: int) -> None:
    os.dup2(source, fd)
    if os.name == "nt":
        import msvcrt
        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        kernel.SetStdHandle.argtypes = [ctypes.c_uint32, ctypes.c_void_p]
        kernel.SetStdHandle.restype = ctypes.c_int
        if not kernel.SetStdHandle(((-10, -11, -12)[fd]) & 0xFFFFFFFF, msvcrt.get_osfhandle(fd)):
            raise ctypes.WinError(ctypes.get_last_error())


def _setup_fds() -> int:
    """Reserve stdout for the protocol; route fds 1/2 through captured pipes."""
    global _protocol_fd, _pump_out, _pump_err, _host_stderr_fd
    _protocol_fd = os.dup(1)
    os.set_inheritable(_protocol_fd, False)
    _host_stderr_fd = os.dup(2)
    os.set_inheritable(_host_stderr_fd, False)
    out_r, out_w = os.pipe()
    err_r, err_w = os.pipe()
    _replace_fd(1, out_w)
    _replace_fd(2, err_w)
    os.close(out_w)
    os.close(err_w)
    sys.stdout = _TaggedWriter("stdout", fallback_fd=os.dup(1))
    sys.stderr = _TaggedWriter("stderr", fallback_fd=os.dup(2))
    stdin_fd = os.dup(0)
    devnull = os.open(os.devnull, os.O_RDONLY)
    _replace_fd(0, devnull)
    os.close(devnull)
    sys.stdin = open(os.devnull, "r")  # user input() sees EOF, never protocol frames
    _pump_out = _Pump(out_r, 1, "stdout")
    _pump_err = _Pump(err_r, 2, "stderr")
    return stdin_fd


def main() -> None:
    global _loop, _serve_task
    stdin_fd = _setup_fds()
    _start_owner_watchdog()

    # Alias the executing module so an in-cell `from vivy_workbench.repl import emit`
    # binds the live module, not a second copy.
    sys.modules.setdefault("vivy_workbench.repl", sys.modules[__name__])
    # A real __main__ module makes dill pickle user functions/classes by value.
    user_module = types.ModuleType("__main__")
    user_module.__dict__["__builtins__"] = __builtins__
    sys.modules["__main__"] = user_module

    _loop = asyncio.new_event_loop()
    asyncio.set_event_loop(_loop)
    queue: asyncio.Queue[dict[str, Any]] = asyncio.Queue()
    signal.signal(signal.SIGINT, _sigint_handler)
    threading.Thread(target=_read_requests, args=(stdin_fd, queue), daemon=True).start()

    from .capabilities import bootstrap, memory_status
    inventory = bootstrap(user_module.__dict__, json.loads(os.environ.get('VIVY_WORKBENCH_SKILLS', '[]')),
                          usage=lambda record: _send({'event':'skill_usage', 'id':_current_cell.get(), 'data':record}))
    _send({"event": "ready", "protocol": PROTOCOL_VERSION, "python": platform.python_version(), "skills": inventory, "memory": memory_status(user_module.__dict__)})

    _serve_task = _loop.create_task(_serve(queue, user_module.__dict__))
    # On the loop, so re-attached shells can deliver their completion notices.
    # A KeyboardInterrupt escaping a cell or background task stops
    # run_until_complete; the interrupt is already recorded, so resume serving.
    while not _serve_task.done():
        try:
            _loop.run_until_complete(_serve_task)
        except KeyboardInterrupt:
            continue
    pending = asyncio.all_tasks(_loop)
    for task in pending:
        task.cancel()
    if pending:
        _loop.run_until_complete(asyncio.wait(pending, timeout=1))
    _loop.close()


if __name__ == "__main__":
    main()

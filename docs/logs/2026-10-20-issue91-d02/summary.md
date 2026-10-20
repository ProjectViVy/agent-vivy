# D02: Byte-framed model completion and request observation

The runtime emits model.completed v3 with exactly byte_len and model.request
v4 with call, route, tool and byte-count metadata. It no longer computes or
publishes request/completion content hashes. The existing Eino v0.9.21 typed
agent/runner and Service.Run/Journal path remain the execution owner.

SDK, TUI, headless and Web validate byte framing without treating completion
as a Run terminal. The non-interactive Face deduplicates accepted event
sequences before rendering or accounting. Web derives framing from accepted
events and preserves displayed text on protocol failure without dispatching a
queued task or fabricating a server terminal.

Historical v2 completion metadata is normalized at RPC egress and message
projection without inspecting the old digest. Original Journal bytes remain
unchanged. Legacy v1 completion can only use its already-persisted message
projection; raw event reconstruction/replay is explicitly unsupported.

The simplest viable design shares stateless historical normalization between
existing runtime/RPC owners and keeps live framing within each Face boundary.
Importing the TUI renderer into facerun was rejected by the standalone-module
check: it required a new Bubble Tea dependency in headless. No new dependency,
runner, persisted state or generic event registry was introduced instead.

The browser smoke used only an isolated home, fresh nonce file, loopback
scripted provider and the split Vite origin. Actual read_file output and its
call ID reached the next provider request; Unicode final text, tool preamble,
completed state and transcript after reload were observed.

D01 blockers remain; D02 acceptance is open until required verification and
conformance regeneration pass. No database migration, production Journal,
D03 work, nudge/warming/steering extraction, release or push is included.
The changed standalone headless test requires its existing Go/YAML source
identity declarations to be recomputed using the repository tool. No
conformance pass record is edited to match an unverified candidate.

Independent review found three stream-error gaps. The single fix pass now
reports permanent failures to headless, stops permanent Web/TUI recovery and
notifies catch-up replay failures. An optional retryable:false field extends
existing run/stream_error; normal transport recovery remains unchanged.

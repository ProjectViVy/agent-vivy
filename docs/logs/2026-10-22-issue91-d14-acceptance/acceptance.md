# Human real-model acceptance — pending

Automatic gates cover executable `09aad07df0acd61c48c51d52f70a59ef44698bd4`; the closing record-only
commit does not change it. The human verdict is deliberately left open.

The actual native minimal-chat Recipe is packed and inspected, and its tiny UI
is staged using Inspect's exact Generation ID. Split Vite serves tiny Face at
`http://127.0.0.1:3015`; the packed native backend listens on loopback8787.
The independent Windows checkout's `.workspace/d14-manual/` holds the disposable
home/workspace, nonce file, actual inspected identity and manual-session.json.
The bound Session is created through existing session/create before testing.
No production database or workspace is used.

Open the existing settings page at `http://127.0.0.1:3016/settings?tab=model`
as an auxiliary developer client of that same backend. It retains its truthful
built authoring-default UI identity; it is not stamped as the tiny Generation
and its unrelated capabilities are outside this acceptance. Vite preview serves
the actual default UI from native CI while3015 serves the matching staged tiny
UI, avoiding simultaneous dependency optimization of two Assembly graphs.

Enter the provider credential only in that local settings owner. No key belongs
in this chat, source, screenshots or verification logs. Save the configuration,
close that page, then refresh3015 while idle.

Suggested prompt: "List this workspace, read nonce.txt, and quote its contents
in your final answer." Do not include the file's value in the prompt. Confirm
real protected list_dir/read_file output, exact result/call identity in the
continuation, one completed terminal and an idle Run. Send a follow-up, cancel
an active request, and confirm the next message still works. Each cancellation
must leave one terminal and no active Run.

The isolated native backend is healthy at `http://127.0.0.1:8787/healthz`
(HTTP200). The matching tiny UI at `http://127.0.0.1:3015` and the auxiliary
model settings page at `http://127.0.0.1:3016/settings?tab=model` both return
HTTP200. Session creation succeeded through the existing RPC path, and the
current session is bound to the disposable workspace. The packed and inspected
Generation is `b521a37913fdfed6c8e245da5a956ae4`. `nonce.txt` exists in that
workspace. Setup issued no Run or provider request; no credential is configured.
Both pages are ready for the owner's manual real-model pass.

Record the human result and redacted Run/call/result identities before marking
D14 or issue91 complete. A successful loopback fixture is automatic evidence,
not this real-model human verdict. No issue closure, push, publication,
stable-root merge or tenant cutover follows from opening these pages.

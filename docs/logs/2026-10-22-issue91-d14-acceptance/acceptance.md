# Human real-model acceptance — pending

Automatic gates cover executable `5b0b572da80602f46d7496d004aa9d88fc6f5eeb`;
the record-only delivery commit does not change it. The human verdict is the
only remaining D14 gate.

The actual native minimal-chat Recipe is packed and inspected, and its tiny UI
is staged using Inspect's exact Generation ID, `6b41cd840bde56a3e97312ed00674327`.
Split Vite serves tiny Face at `http://127.0.0.1:3015`; the packed native
backend listens on loopback `8787`. The independent Windows checkout's
`.workspace/d14-manual-5b0b572d/` holds the disposable home/workspace, nonce
file, inspected artifact and manual-session.json. Session
`sess_c5533c0a93858efa` was created through existing session/create and is bound
to that disposable workspace. No production database or workspace is used.

Use the visible settings page at `http://127.0.0.1:3017/settings?tab=model`
as an auxiliary developer client of that same backend. It has a fresh browser
origin to avoid stale local workspace selection and retains its truthful built
authoring-default UI identity; it is not stamped as the tiny Generation, and its
unrelated capabilities are outside this acceptance. Vite preview serves the
default UI built by native CI while `3015` serves the matching staged tiny UI.
The DeepSeek OpenAI-compatible provider panel is expanded and its API Key field
is blank; choose another provider or add a custom provider if that matches your
account.

Enter the provider credential only in that local settings owner. No key belongs
in this chat, source, screenshots or verification logs. For the expanded
DeepSeek panel, leaving the API Key field saves it to the disposable local
runtime; the field is write-only. Click the tiny Face tab to leave the field,
then refresh `3015` while idle. New model requests see a saved key immediately;
the setup itself sends no provider request.

Suggested prompt: "List this workspace, read nonce.txt, and quote its contents
in your final answer." Do not include the file's value in the prompt. Confirm
real protected list_dir/read_file output, exact result/call identity in the
continuation, one completed terminal and an idle Run. Send a follow-up, cancel
an active request, and confirm the next message still works. Each cancellation
must leave one terminal and no active Run.

The isolated native backend is healthy at `http://127.0.0.1:8787/healthz`
(HTTP200). The matching tiny UI at `http://127.0.0.1:3015` and the fresh-origin
model settings page at `http://127.0.0.1:3017/settings?tab=model` both return
HTTP200. Session creation succeeded through the existing RPC path, and the
session is bound to the disposable workspace. The packed and inspected
Generation is `6b41cd840bde56a3e97312ed00674327`. `nonce.txt` exists in that
workspace. Setup issued no Run or provider request; no credential is configured.
Both visible pages are ready for the owner's manual real-model pass.

Record the human result and redacted Run/call/result identities before marking
D14 or issue91 complete. A successful loopback fixture is automatic evidence,
not this real-model human verdict. No issue closure, push, publication,
stable-root merge or tenant cutover follows from opening these pages.

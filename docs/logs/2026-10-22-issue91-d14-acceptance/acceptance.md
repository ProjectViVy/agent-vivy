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
The direct Python SDK smoke succeeded, but the Vivy attempt through the custom
provider `Gemini 3.8 local gateway` failed before sending a request to the
gateway: the embedded runtime reported `no embedded vendor data`. Do not use
that custom provider for a Vivy run. Relaunch the isolated Vivy process with
its embedded OpenAI-compatible provider frozen to the local gateway:
`VIVY_PROVIDER=deepseek`, `VIVY_API_BASE=http://127.0.0.1:8045/v1`,
`VIVY_MODEL=gemini-3.8-flash-tiered`, and `DEEPSEEK_API_KEY` supplied through a
hidden local prompt. Keep `VIVY_USER_HOME` on the disposable D14 home. Never put
the key in chat, a command line, a file, a screenshot, or a verification log.

After that isolated backend restart, refresh the tiny Face page at `3015` while
idle. Its draft is prefilled with the workspace list/read acceptance prompt;
send it only after the process is healthy. The earlier full-shell attempt at
`3017/` is not the Tiny Face acceptance run.

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
workspace. Three Vivy runs were attempted from the full UI; all failed before
completion, and none reached the gateway. No successful Vivy model call or
Tiny Face tool/action acceptance has occurred. The custom-provider key field
is write-only and was not re-read after the manual attempt.

Record the human result and redacted Run/call/result identities before marking
D14 or issue91 complete. A successful loopback fixture is automatic evidence,
not this real-model human verdict. No issue closure, push, publication,
stable-root merge or tenant cutover follows from opening these pages.

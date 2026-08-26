# Acceptance — 2026-08-26 list_dir tool

How a human can tell it worked, from the product view:

1. **Dev loop**: start `just dev`, open `http://127.0.0.1:3015`, and ask the
   agent something like "看看这个工作区里都有什么文件" (look at what files are in
   this workspace). With a configured model, the agent now has `list_dir`
   available and can answer by listing the run workspace tree — including
   bounded recursion (`recursive: true`, `depth`) — instead of being stuck
   between `read_file` (needs a known path) and `search_files` (needs known
   text).
2. **No approval friction**: `list_dir` is readonly; turns using only it run
   without any approval prompt (preflight smoke showed
   `decision: allow, reason: readonly tool is allowed by default`).
3. **Config surface**: `config.example.yaml` documents `list_dir` in both
   `tools.enabled` and `runtime.sandbox.approval.auto_approve_tools`; an
   operator can remove it from `tools.enabled` to disable it, or add it to
   `auto_approve_tools` to keep it auto-approved under the `auto` policy.
4. **Safety unchanged**: listings stay inside the per-run workspace; escapes,
   symlinked entries, and protected paths (`.env`, `keys.txt`, ...) are
   rejected exactly like reads; `.git`/`node_modules` never get traversed in
   recursive mode; oversized results come back with `truncated: true` instead
   of blowing up the context.

Not part of this delivery: Eino middleware surface changes, UI changes, and
the remaining agentg gap items beyond A.

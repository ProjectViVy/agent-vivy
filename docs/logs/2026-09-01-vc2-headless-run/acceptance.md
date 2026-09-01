# Acceptance — VC-2 headless `vivy run`

How a human can tell it works:

1. Run `vivy run --help` — usage text appears with exit code 0.
2. `echo "prompt" | vivy run` or `vivy run "prompt"` with a configured
   provider: assistant text streams to stdout; tool activity
   (`> tool_name`), failures, and the final verdict go to stderr only.
   Exit code is 0 on completion, 1 on failure, 2 on Ctrl+C or a
   cancellation.
3. `vivy run --continue "follow-up"` attaches to the most recent session
   instead of minting a new one; the session title of a fresh run is the
   prompt truncated to 60 characters.
4. In the web UI (`http://127.0.0.1:3015`), a headless-created session
   appears with its journal intact — messages, tool calls, and the run
   verdict are visible in the same history the web face uses.
5. Headless cannot ask for approval: when a tool call requires human
   approval (or the model asks a question), stderr prints a loud
   `vivy: blocked: ...` notice, the run cancels with exit code 2, and
   nothing is silently auto-approved. Approving the same tool in the web UI
   or relaxing the permission preset is the intended remedy.
6. While a normal `vivy serve` gateway holds the lease, a concurrent
   `vivy run` fails cleanly instead of corrupting state (single-organism
   lease), and settings-overlay compaction values apply to headless runs
   the same as web runs.

Contract notes: behavior aligned with FACE-0 (`VIVY-FACE-PACK.md` §5,
§14.4) and the D11 ruling; Crush is FSL-1.1-MIT, so this slice reproduces
protocol/behavior alignment only — no Crush source was copied.

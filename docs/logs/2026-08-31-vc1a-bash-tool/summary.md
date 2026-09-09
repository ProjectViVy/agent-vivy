# VC-1a: bash tool (mvdan shell parsing + tiered approval + deny list)

## Delivery scope

Added the built-in `bash` tool to match Crush's shell capability surface: the model provides a shell
script, and the tool executes it with `bash -c <script>`. The security model has three layers:

1. **Deny list (policy-independent hard refusals).** Fork bombs, mkfs/format/diskpart/
   fdisk, shutdown/reboot/halt/poweroff, recursive deletion of system roots (`/`, `/usr`, `~`,
   `$HOME`, `C:\`, etc.), `dd of=/dev/{sd,hd,vd,nvme,disk}`,
   redirection into raw disk devices, and `curl|wget … | sh` pipeline execution are refused.
   A match is rejected; no profile, including full_auto, can bypass it.
2. **Tiered approval (`InvocationClassifier`).** The tool-level `Readonly` flag only describes
   whether "this tool changes anything"; `bash` parses the script AST with mvdan.cc/sh/v3
   and classifies each invocation further:
   - `safe`: read-only commands (cat/ls/grep/rg/jq/sort, etc.), read-only git subcommands
     (status/log/diff/show/blame, etc.), `go env/list/version`, and their pipeline combinations;
     redirections whose output goes only to `/dev/null`, stdout, or stderr also count as safe.
   - `mutating`: everything else (write redirections, expanded variable command names, unknown commands, etc.);
     the existing approval path is unchanged.
   - `denied`: a deny-list match.
   The approval gate classifies the **final arguments** (after hooks rewrite them). Under the `auto` approval policy,
   safe calls execute directly and emit a governance event (reason =
   "safe read-only invocation auto-approved"); `ask`/`never` behavior is unchanged, and the deny list still rejects under full_auto.
3. **Sandbox handoff.** `bash` is intercepted in `validateRequest` and uses dedicated validation
   (the host must have bash, a read_only sandbox rejects it, the arguments must be exactly
   `["-c", script]`, and the deny list is checked a second time for defense in depth); it no longer goes through the executable
   allowlist. Normal `execute`/`commandline` paths are unaffected.

Other deliverables:

- `tools.InvocationClassifier` is now a general extension point for the approval gate; its default
  zero value is `InvocationMutating` (fail-closed).
- `ValidateArgsSafety` exempts shell-syntax token blocks (`&&`, `|`, `$(`, etc.) in the `bash` `command`
  argument; the NUL check remains and always runs.
- Extracted the command backend's workspace cwd / environment sanitization / timeout-clamping logic into the shared
  `resolveCommandContext`.
- Added `bash` to the default enabled surface (config).
- `bash.Spec` documents the tiered semantics in its description; `PrepareProposal` presents the script
  as the proposal preview, and `RiskFindings` marks "command output is untrusted".

## Explicitly not done (see docs/TODO.md)

- Background job registry (job_output/job_kill) → VC-1b.
- Bash output truncation still uses tooladapter's compactToolResult budget; there is no
  independent head/tail budget tuning.
- Native Windows PowerShell/cmd is outside this tool's scope (bash only; a host without
  bash reports "bash is not available on this host").

## Behavior alignment (with existing principles)

- "Match Crush's functionality; do not add what it does not have": the bash tool and tiered approval are part of
  Crush's existing capability surface (shell tool + command allowlist/tiering); the deny list uses a common-sense set
  of hard-dangerous operations and adds no extras beyond Crush.
- FSL-1.1-MIT: only Crush's behavioral semantics were studied; no code was copied. The parser is
  mvdan.cc/sh/v3 (MIT).

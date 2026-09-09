# 2026-09-01 — VC-3 slice 1: proc.spawn capability + LSP plugin (`lsp_diagnostics`)

## What changed

This is the first deliverable slice of VC-3 (D4: LSP = an independent vivy-sdk
module plugin using the tool_world seam), delivered as one capability + consumer
pair:

### Kernel side: proc.spawn capability (kernel/SDK)

- `sdk/plugin/plugin.go` — new `proc.spawn` grant; `SpawnSpec`/`Proc` types;
  `plugin.Env` adds a fifth method `Spawn(ctx, spec) (Proc, error)`. `os/exec`
  remains blocked in plugin source—spawn is a kernel-hosted capability, analogous
  to Listen in ChannelHost. The child-process cwd is fixed to the plugin
  workspace, and it outlives a tool call (the plugin is responsible for it until
  Close).
- `sdk/internal/manifest.go` — proc.spawn may be declared only by the tool_world seam
  (declaring it on the tool seam makes verify fail), matching the seam restriction for
  channel-family grants.
- `sdk/internal/testdata/bad-procspawn-seam/` + `verify_test.go` — negative fixture
  and assertion.
- `internal/pluginhost/host.go` — `hostedEnv.Spawn` implementation: grant
  fail-closed; command = bare PATH name or workspace-relative path (absolute
  paths/escapes rejected); `exec` uses `context.WithoutCancel` (retains context
  values while dropping run cancellation, because language servers must survive
  across calls); stdin/stdout/stderr are three pipes; `Close()` = kill + reap.
- `internal/pluginhost/host_test.go` — real subprocess tests (echo/cd/pwd prove pipes
  and the fixed workspace cwd; no-grant rejection; escape rejection; Close kills the child).

### Plugin side: plugins/lsp (independent module, D4 first example)

- `plugins/lsp/` is an independent module (`module example.com/vivy/plugins/lsp` +
  `replace agent-vivy => ../..`), with zero third-party dependencies (including no new go.sum entries).
- `jsonrpc.go` — hand-written LSP base-protocol codec (Content-Length frames,
  skipping Content-Type). The research item evaluated a powernap (MIT) fallback
  and estimated 1–2k lines of hand-written code; the actual hand-written subset
  needed by this plugin is ~110 lines, so powernap was not introduced—zero new
  supply-chain dependency.
- `protocol.go` / `languages.go` / `client.go` / `manager.go` / `plugin.go` —
  LSP client (initialize/initialized, full-text didOpen/didChange sync,
  publishDiagnostics capture and "new publication" waiting semantics); manager
  lazily starts by (language, workspace root), replaces dead processes, and
  reaps idle servers (10 minutes, checked every minute—the plugin contract has no
  Stop hook, so reaping is the close story); `lsp_diagnostics` tool (effect read,
  content-only: env.OpenRead → didOpen/didChange → wait for fresh publication →
  format `path:line:col: severity: message [source]`, wait_ms default 3000,
  maximum 15000, timeout returns existing results with a note).
- `vivy-plugin.json` — tool-world seam, grants [fs.read, proc.spawn],
  tools [lsp_diagnostics].
- `plugin_test.go` — deterministic end-to-end tests with no real-server dependency:
  fakeEnv.Spawn starts an in-memory fake language server (io.Pipe + real jsonrpc
  frame protocol), covering initialize → didOpen → publish → formatting end to end,
  connection reuse on the second call (didChange path), and argument validation that
  does not trigger spawn.

## Verification command

See `verification.md`.

## Explicitly not done (not in this slice)

- **Post-edit diagnostic backfill** (attach LSP diagnostics to write/patch/multiedit results) —
  requires a design connecting the kernel write path to plugin diagnostics (an implementation
  design point in the VC-3 line); deferred to the next slice.
- **lsp_definition / lsp_references / lsp_symbols / lsp_rename** tool family —
  to follow in batches; rename goes through write approval.
- **File-version history** (L2 session-level rollback) — attached to the RB-1 line,
  pending the user's decision on O1..O6.
- **UI file preview / syntax highlighting / read_file images**.
- **powernap evaluation** — the hand-written client covers the need, so the evaluation item
  is void (zero new supply-chain dependencies).
- Diagnostic positions use LSP UTF-16 code units (the protocol default); no encoding negotiation
  (`positionEncoding`) or UTF-8 conversion was implemented — line numbers match in gopls scenarios,
  while columns may differ; revisit in a later slice.
- Diagnostics include only publications for the tool call's target URI (the server may publish
  other files, which are not displayed for now).

## Crush alignment

Crush is FSL-1.1-MIT: this slice is behavior/protocol alignment (LSP diagnostics
as coding-quality feedback), with zero code copied. `lsp_diagnostics` is an
existing Crush behavior (LSP diagnostics + lint/type errors delivered directly
to the model); this slice adds no surface that Crush lacks. Diagnostic backfill
(Crush's key mechanism) belongs to the next slice.

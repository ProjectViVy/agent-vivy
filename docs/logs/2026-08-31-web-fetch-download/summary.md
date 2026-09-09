# 2026-08-31 — web_fetch + download tools (CRUSH parity alignment)

## What changed

The Vivy kernel adds two network tools to close the capability gap with CRUSH `fetch`, while retaining Vivy's
security model (CRUSH has approval only and no SSRF protection):

- **`web_fetch` (read-only, no configuration)** — `internal/tools/web_fetch.go` +
  `internal/runtime/web_fetch.go` (`EinoWebFetchBackend`). GET-fetches any public
  URL and outputs one of three formats, `markdown | text | html` (markdown by default, aligned with CRUSH).
  Content-Type routing: text/html → goquery strips noise (script/style/nav/header/footer/
  aside/noscript/iframe/svg/template) → html-to-markdown conversion; JSON → pretty-printed;
  text/* passes through; binaries are rejected with a prompt to use download. Non-2xx responses follow DSH convention and return bounded results
  rather than errors (error pages often contain readable information). The response cap reuses `runtime.http_max_response_bytes`
  (1MB by default); over-limit responses are truncated with a `[Content truncated to N bytes]` marker (CRUSH-style,
  without a hard error).
- **`download` (effectful, approval required)** — `internal/tools/download.go` +
  `internal/runtime/download.go` (`EinoDownloadBackend`). Streams a URL to the current run workspace
  (CRUSH alignment: binary-safe, overwritable, and automatically creates parent directories); uses the existing
  D-012 approval gate (`ProposalProvider.PrepareProposal` → HITL interrupt →
  precondition hash for stale protection), with a hard 100MB limit and SHA256 returned.
- **Security gate (shared by both tools; all parts absent from CRUSH are retained)**:
  - `publicOnlyDialContext`: after DNS resolution, rejects loopback/private/link-local addresses one IP at a time;
    **there is no `http_request` localhost exemption**—no allowlist fallback, and private targets are always rejected
    at dial time (DNS rebinding protection).
  - D-021 `sandbox.CheckNetwork` integration is unchanged.
  - Absolute HTTP(S), URLs with embedded credentials, and credential-shaped query parameters are rejected; redirects are rechecked hop by hop,
    with a maximum of 5 hops.
  - Results always have `Untrusted: true` and go through the existing `[UNTRUSTED TOOL OUTPUT]` normalizer.
- **Registration**: `internal/tools/tools.go` adds `BuiltinWithWeb` (a new thin-shell chain stage;
  `BuiltinWithCommands` semantics are unchanged), and `baseToolsForSearch` indexes it as well; `Default()`
  adds both tools to `Tools.Enabled`; `config.example.yaml` documents the security boundary.
  **No new configuration keys** (the target is zero-configuration).
- **Dependencies**: adds `JohannesKaufmann/html-to-markdown v1.6.0` (MIT),
  `PuerkitoBio/goquery v1.12.0` (BSD-3), and indirect `x/net v0.52.0`/
  `cascadia v1.3.3`; versions are aligned with CRUSH's go.mod. The plugin modules (discord/qq) ran `go mod tidy`
  once because the root go.mod was upgraded.

## License boundary

CRUSH is FSL-1.1-MIT: this iteration provides **behavioral alignment only** (parameter surface, format enum, truncation marker,
timeout clamp, and download semantics); all code was written from scratch and no CRUSH source was copied.

## What was explicitly not done

- `agentic_fetch` / `sourcegraph` (remaining CRUSH parity items, recorded as TODO §0.1 **WEB-1**;
  the research document assigns them to optional VC-4).
- CRUSH's `web_search` subagent variant (Vivy already has `network_search`, so it is not duplicated).
- No semantic changes to `http_request`—the allowlisted tool remains as-is, and the division of responsibilities is unchanged.
- New configuration keys or a UI settings surface (the target is zero configuration; download approval uses the existing general HITL UI).

## Findings filed during the iteration

- **WEB-2** (TODO §0.1, OPEN): existing `WriteFile` sandbox validation runs before
  `MkdirAll`, so restricted-mode writes to a new nested directory are falsely rejected; this iteration self-fixed download with
  "resolve → MkdirAll → Validate" and a regression test, while WriteFile remains to be fixed.

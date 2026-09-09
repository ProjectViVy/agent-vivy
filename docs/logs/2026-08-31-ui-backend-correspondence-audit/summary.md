# UI ↔ backend correspondence audit

## Changes

- Added `docs/research/ui-backend-correspondence-2026-08-31.md`.
- Completed a bidirectional scan of the non-channel Web UI against the real JSON-RPC/runtime/product contract.
- Recorded 10 findings where the backend exists but the UI is mismatched or incomplete; explicitly excluded demo surfaces that do not yet exist in the backend.

## Scope

Covered the chat run path, Skills, Dashboard, Lifecycle, Review Center/Run Inspector, context compaction, primary navigation, and correspondence checks for Provider/MCP/Token/Session. Product code, channels, and Studio were not modified.

## Not completed

This delivery is an audit report and does not implement the UI fixes identified in the report. The browser runtime currently has no usable instance, so a visual screenshot smoke test could not be completed; the HTTP split processes started and returned 200.

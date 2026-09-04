# VIVY CODE approval diff

## Changed

- Preserved the server-authoritative approval action, target, bounded unified preview, and risk findings through the shared stream projection.
- Added a single shared approval-diff surface for built-in and packed TUI faces: split at wide viewports, unified on narrow viewports, fullscreen toggle, bounded vertical/horizontal scrolling, line numbers, and visible-preview add/remove counts.
- Matched the applicable Crush interaction contract: `t` toggles split/unified, `f` toggles fullscreen, Enter/Ctrl+Y/`y` approves, and Esc/`n` denies.
- Sanitized terminal controls and bidi controls, redacted absolute targets, reset view state for each new gate, and defensively copied risk slices at both live-face boundaries.
- Made preview display preview-first in fullscreen, plain REPL, and existing tool cards; late approval/question RPC responses are fenced by gate, session, and run identity.
- Bounded approval review payloads before Journal append, explicitly marked truncation, counted every new field against the client inbox budget, and bounded risk count/bytes.
- Reused that same bounded Review metadata for Approval storage and Journal replay, hid raw `tool.requested` arguments, bounded complete `tool.finished` payloads including Parts, and decoded structured mutation results only behind the exact runtime untrusted-output envelope.
- Applied the runtime's single-source secret redactor to approval target/preview/risk metadata before storage or Journal publication. Raised the supported event-payload minimum to 1 KiB and pinned both approval and tool-result envelopes to that hard encoded ceiling.

## Explicitly not done

- No diff is reconstructed from tool arguments, current workspace files, or non-file tool previews. A recognized file-mutation action plus a valid server unified preview is required.
- No “allow for session” action is shown because the approval RPC does not provide that decision.
- This delivery does not add the remaining token/cost presentation work in `TUI-PARITY-2`.

This is a focused source delivery, not a release; no release record is included.

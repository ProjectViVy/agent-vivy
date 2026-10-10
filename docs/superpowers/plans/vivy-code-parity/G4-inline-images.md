# G4 — Inline terminal images (kitty/iTerm2) [risk-flagged]

**Goal:** assistant/attachment images render inline in supporting terminals; graceful degradation elsewhere.
**Epic:** G. **Requirements:** RQ-TUI (partial — pi does this via kitty/iTerm2 auto-detect).
**Spec:** VCP-D1 §5.8. **Risk:** highest TUI item; may be descoped to "attach-only" without breaking parity acceptance (index note).

## Scope

**Files:** `sdk/tui/view` image cell writer (raw escape sequences to tty, bypassing bubbletea rendering for the image rows), capability probe (kitty/iTerm2/WezTerm detection via env + query).

## Tasks

- [ ] Probe: detect kitty (`TERM`/`KITTY_WINDOW_ID`/query), iTerm2 (`TERM_PROGRAM`), else fallback.
- [ ] Writer: emit kitty `a=T` chunks or iTerm2 OSC 1337 `File=` with sizing; reserve N rows in transcript; images under attachments + paste-guard path render inline.
- [ ] Flag `images: auto|on|off`; off = current chip behavior.
- [ ] Tests: sequence generation per protocol + fallback path; visual verification manual (kitty terminal) recorded as evidence or marked untested-with-reason.
- [ ] Commit `feat(tui): inline image display for supporting terminals` (or document descope).

## Boundary

No mermaid. No Sixel (document why if asked — kitty/iTerm2 covers pi's stated targets).

## Acceptance

Image attachment renders inline in kitty; a plain xterm shows the old chip.

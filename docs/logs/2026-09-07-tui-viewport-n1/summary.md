# TUI-VIEWPORT-N1-OPEN — Open-state viewport closeout (anchoring + render cache + loading state)

Date: 2026-09-07 · Scope: `sdk/tui/surface`, `sdk/tui/live`, `sdk/tui/view`

## Changes

Three orthogonal closeouts, all in the open-state (active-session) chat viewport:

1. **Paused viewport anchoring (message anchor)**
   - When following stops (PgUp / mouse wheel / PgDn), the viewport-top position
     is no longer stored as a purely numeric offset; it is captured as a
     `(segment, line)` anchor plus the assembly fingerprint at that time
     (`chatAnchorSeg / chatAnchorOff / chatAnchorStamp`, model.go).
   - When the assembly fingerprint changes (content changes), `clampChatScroll`
     remaps the anchor to the current flat offset: adding or removing lines
     above the anchor (such as expanding a tool-result card) no longer makes the
     viewport drift. When the fingerprint is unchanged (pure clamp), it does
     not overwrite an explicitly written numeric offset (preserving the direct
     positioning semantics of `TestChromeScrollHintHiddenNearBottom`).
   - The anchor is correctly set and cleared with `Home` / `End` / bottom
     following / session switches.

2. **Fingerprinted segmented render cache (`chatAssembly`)**
   - `chatSegments` (render.go) renders each message as an independent segment
     (separator rows are built into every segment except the last). A maphash
     fingerprint (ID/Role/Content/all Tool fields/Streaming/Reasoning/
     Attachments/FileContexts + message count) determines whether the whole
     assembly can be reused. `renderChat` now only slices and joins the visible
     window, instead of re-rendering every frame and flattening the entire
     history. mdCache (TUI-MD-STREAM-CACHE) continues to cover per-message
     rendering on the rebuild path.
   - Model uses value receivers: the assembly pointer is allocated once in
     `New()` and updated in place, so value copies share the cache.

3. **History-loading state (`Meta.Loading`)**
   - `surface.Meta` adds `Loading bool`; Live sets it during the
     delete→switch→load window (`loadPending`). The empty-session view renders
     the “Loading session history…” line during this period instead of pretending to
     be the empty-conversation hero (“Journey to Find Your True Heart”).

## Explicitly not done

- Did not change projection order/source columns (`TUI-PROJECTION-ORDER` remains
  on the board and is intentionally last).
- Did not touch the sidebar MCP/LSP status row (`TUI-SIDEBAR-N1-OPEN` is handled
  separately).
- Did not add incremental rendering (dirty-segment re-rendering); a full
  fingerprint comparison is already O(history), rather than O(render), and is
  sufficient.

## Eino capability check

This slice is purely in the TUI presentation layer (surface/live/view); it does
not involve LLM runtime, orchestration, prompts, the streaming pipeline, or
context management. There is no Eino reuse point, and the import-isolation
boundary is untouched.

# D3 — GUI chat-level compact control

**Goal:** compact trigger reachable from chat (not only Settings card) with optional instructions input.
**Epic:** D. **Requirements:** RQ-CMP, RQ-GUI. **Predecessor:** D1 (instructions plumbing; may absorb this story if done there — see D1 plan).
**Spec:** VCP-D1 §5.5.

## Scope

**Files:** `ui/src/components/chat/` composer-area control or a `/compact` slash hint; api/store already extended in D1 (`compactSession(sessionId, instructions?)`).

## Tasks

- [ ] Chat affordance: compact button in the composer row (context-usage meter nearby is the natural anchor) → small popover with instructions input + "compact now".
- [ ] Disabled while run active (RPC returns 409 anyway — surface it as a toast, not a silent fail).
- [ ] Show last-compaction summary marker in transcript (fold marker exists in data — render it).
- [ ] i18n; vitest; browser smoke.
- [ ] Commit `feat(ui): chat-level compact control`.

## Boundary

No auto-compaction UX changes (settings card keeps policy knobs).

## Acceptance

Compact-with-instructions runs from chat; transcript marks the fold point.

# VCP-D3 — Verification

## Commands

- `pnpm vitest run src/components/chat/ChatInput.compact.test.tsx` — 4/4 pass.
- `pnpm vitest run src/lib/run-rows.test.ts src/components/chat/ChatInput.compact.test.tsx src/components/chat/ChatView.test.tsx` — 30/30 pass.
- `pnpm vitest run src/components/chat/ChatInput src/i18n` — 78/78 pass.
- `pnpm typecheck` — clean (tsc --noEmit; staged assembly regenerated).

## New tests

- `ChatInput.compact.test.tsx` (4): control hidden when compaction
  disabled; instructions forwarded to `context/compact`; blank field omits
  `instructions`; trigger stays enabled during a run and the busy error
  surfaces in the composer notice.
- `run-rows.test.ts`: `auto_retry.started`/`finished` fold into `retry`-tag
  notice rows; compaction notice keeps `compact` tag.

## Deferred

Browser smoke folded into H1 acceptance (split pair verified at
127.0.0.1:3015 during that story); `just ci` deferred per-story.

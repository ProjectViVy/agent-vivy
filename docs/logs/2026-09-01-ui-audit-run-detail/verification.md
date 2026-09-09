# Verification

## Gates

- `just ci` — passed (exit 0): Go fmt/vet/test, headless compile, six plugin-ci
  modules, and UI install + `tsc --noEmit` + `vitest run` + `vite build` all
  green (`openSeq` state, `aria-expanded` expansion, and payload `<pre>` pass
  type and lint checks).
- `just ui-e2e` — passed (exit 0, 10 passed / 1 skipped, 26.3s): real browser +
  real control plane; `runtime.spec.ts` covers the main chat-page path
  (including run-event rendering), and the event-row DOM change (button wrapper +
  expandable pre) did not regress existing assertions.

## Smoke notes

- There is no component-specific spec for Inspector event expansion/collapse;
  following the CH-C1-N3 precedent, the full e2e suite is the smoke substitute
  (Run Inspector is on the chat page, and the runtime spec builds from the source
  containing this change). The expansion behavior is available for manual review
  in acceptance.md.
- WebSocket RPC transport (`/rpc/bootstrap` → WS upgrade) has no curl smoke
  path.

## Static review evidence

- `ui/src/components/chat/RunInspector.tsx`: event rows changed from
  `title={JSON.stringify(...)}` to `<button aria-expanded>` (keyboard-accessible);
  the expanded state renders `JSON.stringify(event.payload ?? null, null, 2)`
  in a height-limited pre (`max-h-48` scrolling), and `openSeq` toggles rows
  independently.
- No new i18n keys: payload is structured data printed directly as JSON, with no
  copy.

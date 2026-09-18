# Verification — Vivy UI markdown + empty bubbles

Environment: Windows, split dev pair already running (`vivy` backend
`127.0.0.1:8787`, Vite dev UI `127.0.0.1:3015`), Studio on `:3090`.

## Unit / static

| Command | Result |
| --- | --- |
| `cd ui; pnpm add remark-gfm @tailwindcss/typography` | OK — `remark-gfm 4.0.1`, `@tailwindcss/typography 0.5.20`; `pnpm-lock.yaml` updated |
| `cd ui; pnpm typecheck` | OK (`tsc --noEmit`, exit 0) |
| `cd ui; pnpm test` | OK — 39 files / 338 tests passed, including the 6 new `MessageBubble` cases |
| `just ui-ci` (repo root, final revision) | OK — frozen-lockfile install, `tsc --noEmit`, 338 tests, `vite build`, i18n completeness and cross-face checks |
| `just headless-compile`, `just plugin-ci` | OK — `cmd/vivy`, `cmd/vivy-code`, `ui` compile; every plugin/face module vets and tests green |
| `just ci` (repo root) | **Red, and not from this change** — `sdk/internal/conformance` `TestCheckedInProviderConformanceMatchesExecutedSuites` fails; every other recipe and package is green (`internal/...`, `ui`, `ui` vitest, plugins, faces, `sdk/...`). Attribution below. |

### `just ci` failure attribution (pre-existing, another lane)

The failing test byte-compares the checked-in conformance artifact with the
digests and suites executed from the live tree. The diff is 75 entries across
exactly the five providers whose `SourceRoot` is `internal/`
(`vivy/protected-tools`, `vivy/mcp-host`, `vivy/provider-profiles`,
`vivy/context-source`, `vivy/skill-source`), all of them only in
`sourceSha256`; no suite changed result and no entry is missing.

`internal/sourcehash.Tree` hashes every regular file under the root, so the
untracked work in this tree moves that digest. Measured directly:

```text
live internal/                                     ca4351b056606a2e47c62e926646b13509eee72a45aeef9107e690fc04e85cfd
internal/ minus untracked internal/workflow/ and
internal/domain/workflow_test_support.go           5e386f84276e4d04bcaae93749638a304fde44e8dd3a2f749cbbdc538183d3da
checked-in conformance_results.json               5e386f84276e4d04bcaae93749638a304fde44e8dd3a2f749cbbdc538183d3da
```

The checked-in artifact therefore matches HEAD and the two untracked files are
the whole delta; the UI change touches no file under `internal/` and the
`fixture/full-ui` digest did not drift. No action was taken: that work is
uncommitted in another lane, and its landing owes a conformance-artifact
refresh.

## Unit tests added

New unit tests (`ui/src/components/chat/MessageBubble.test.tsx`):

- an assistant message with empty content renders nothing;
- an empty assistant message still renders while streaming (`…`);
- a user message with empty text still renders (attachment-only turns);
- a GFM pipe table produces `<table>` / `<th>` / `<td>` and no raw `|` text;
- `**bold**` and `` `code` `` produce `<strong>` / `<code>`.

## Live UI (`http://127.0.0.1:3015`, real backend data)

Probe: a throwaway Playwright script (`ui/.dev-workdir/md-smoke.mjs`, gitignored)
driving the installed Chrome against the dev server, selecting the session
`你好` (13 projected messages, 16 tool results, tables in the transcript).

Post-fix measurement (DOM):

```text
articles=13  emptyProse=0  toolCards=16  tables=2  proseWithRawTablePipes=0
```

Same session with the guard line temporarily commented out (A/B probe):

```text
articles=29  toolCards=16  prose=29      (16 extra bubbles — one per tool call)
```

`ui/.dev-workdir/smoke-before.png` shows the reproduced defect: small empty
bubbles each followed by a timestamp + 4-button action bar, one pair above each
tool result card — the same shape as the user's screenshot.
`ui/.dev-workdir/smoke-fixed.png` shows the same view after the fix: no empty
bubbles, markdown list/bold/inline-code and paragraph spacing rendered, GFM
tables present.

### Regression caught by the same live check

The first post-plugin screenshot showed the user bubble repainted: the
typography plugin's `.prose` sets `color: var(--tw-prose-body)`, so white text
on the primary bubble turned grey. A 5x crop of the bubble was compared before
and after the follow-up fix: `.prose-inherit` (colour variables pinned to
`currentColor`) is applied to the user bubble body only, and the crop is white
again. Pinned by a unit test asserting the user bubble body carries
`prose-inherit`.

Dev-server module check (proves the served bundle, not only the source):

```text
GET /src/styles.css?direct            -> 16x ".prose" rules emitted (typography active)
GET /src/components/chat/MessageBubble.tsx
  import remarkGfm from "/node_modules/.vite/deps/remark-gfm.js?v=..."
  <MarkdownBody content={...}> at both bubble call sites
```

## Not verified

- No run was re-executed end to end (no new model turn was spent); the fix was
  checked against transcripts already in the console Journal.
- The typography plugin also affects `PersonaMemoryView` and
  `ChannelTutorialModal`, which use `.prose`; both were previously unstyled by
  the same missing plugin. They were not visually re-inspected beyond the chat
  surface.

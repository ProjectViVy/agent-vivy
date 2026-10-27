# Block 3 — verification

## Code gates (worktree `feat/ui-overhaul`)

```text
cd ui && pnpm typecheck
# exit 0 (runs stage-ui-assembly first; no generated file changed)

cd ui && pnpm vitest run src/components/chat/ToolRow.test.tsx \
  src/components/chat/MessageBubble.test.tsx src/components/chat/ChatView.test.tsx \
  src/components/chat/DeliverableCard.test.tsx src/components/chat/ReasoningRow.test.tsx
# 5 files, 27 tests passed

cd ui && pnpm test
# 88 files, 710 tests: 709 passed, 1 failed
#   src/lib/store.test.ts > keeps the run.failed payload as the conversation error
#   pre-existing and already on docs/TODO.md §0.1 as STORE-RUN-FAILED-COPY
#   (expected the generic connect-failure line, got `provider deepseek: API key missing`)

cd ui && pnpm build
# exit 0, built in 1.61s (existing chunk-size warning only)
```

Token ratchet (`src/theme/tokens.test.ts`) passes with the baselines updated in
this block: `src` palette 154 / text-size 55 / hex 0, `plugins` 29 / 45 / 0. No
file newly violates; the five cleaned transcript files were removed from the
baseline lists.

## Browser evidence

Environment: isolated split pair, not the owner's `:8787`/`:3015`.

```text
VIVY_ADDR=127.0.0.1:8796 VIVY_USER_HOME=/tmp/vivy-ui-overhaul \
VIVY_PROVIDER=deepseek VIVY_API_BASE=http://127.0.0.1:9913/v1 \
VIVY_MODEL=mock-fast DEEPSEEK_API_KEY=mock-key go run ./cmd/vivy   # :8796
VIVY_BACKEND_ADDR=http://127.0.0.1:8796 pnpm exec vite --port 3016 --host 127.0.0.1 --strictPort
```

Mock (`/tmp/vivy-mock-llm.py`, localhost only) routes on the last user message:

| marker | reply |
| --- | --- |
| `VIVY-RICH` | streamed markdown: long prose, nested `tsx` fence, 200-char inline path, table |
| `VIVY-TOOL` | `read_file` call with a 200-char path, then final text after the tool result |
| `VIVY-APPROVAL` | `write_file` call → approval required under the Smart preset |
| last message role `tool` | `Tool finished. DONE` |

Persona initialization gates primary admission in a fresh `VIVY_USER_HOME`; the
capture opened `/persona`, filled the five required documents, and submitted
`Initialize persona` once before the chat states.

Capture (`ui/.audit/capture-block3.mjs`, gitignored; Chrome channel because the
bundled Playwright browsers are not installed on this host) produced, per width
390 and 1280: a 3k-character user paste, the rich assistant answer, a tool row
with the long path, and a `write_file` awaiting-approval row. It reported, per
state, `document.scrollWidth === clientWidth` and no element whose content is
wider than its box, except intentional `sr-only` spans and one composer-side
`truncate` on the workspace chip at 390 (Block 4 scope). Console: no errors.

Artifacts in this directory:

- `transcript-1280-{rich,tool,approval}.png`
- `transcript-390-{rich,tool,approval}.png`
- `overflow-report.json` (per-state overflow measurements)

Before the `scroll-area.tsx` fix the 390 capture showed the transcript content at
822px inside a 390px viewport (`display: table` wrapper), clipping the assistant
markdown and code block. After the fix the same measurement is 390/390 with the
code block scrolling internally and prose wrapping.

## Not run

- `just ci` from the repo root: the kernel gate is not available on this host
  (recorded for every block; the UI lane gates above are the ones that ran).
- Real-provider streaming and packaged/embedded UI (`:8787`) — out of scope for
  this block.

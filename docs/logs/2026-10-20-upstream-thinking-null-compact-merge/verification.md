# Verification — upstream thinking-level null fix and compact control

Environment: Windows, `go1.26.4`, `node v24.19.0`, `pnpm 10.11.0`,
`just 1.46.0`. All commands ran from the repository root.

## Focused tests

```text
go test ./internal/rpc -run 'Thinking' -count=1
→ ok agent-vivy/internal/rpc 8.129s

pnpm --dir ui exec vitest run \
  src/components/settings/ThinkingSettingsCard.test.tsx \
  src/components/chat/ChatInput.compact.test.tsx
→ 2 files, 7 tests passed (6.2s)
```

## Full UI gates

```text
pnpm --dir ui test
→ 80 files, 629 tests passed (34.8s)

pnpm --dir ui typecheck
→ tsc --noEmit, exit 0
```

## Real split-path smoke

The split pair was restarted from the merged tree (`just dev`, backend
`:8787`, Vite `:3015`) with a fresh isolated data root, then exercised with a
headless Playwright page:

- `/settings` rendered without `Vivy UI Module failed`; the thinking card was
  present and no page or console errors were recorded.
- `/` exposed one `上下文占用` button; clicking it opened the compact popover
  containing `立即压缩`, and Escape closed it.
- Backend startup logged `settings overlay applied … compaction_enabled:true`.

## Not run

- `just ci`: the owner stopped the earlier run and did not ask for a new full
  gate. The focused Go test, full UI suite, typecheck and live split smoke are
  the evidence for this merge.
- `ui/e2e/runtime.spec.ts` was updated by the upstream compact commit but was
  not rerun after the cherry-pick; its compact assertions were covered by the
  live smoke above.
- No live provider request or real API key was used.

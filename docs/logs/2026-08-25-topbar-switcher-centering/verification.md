# Verification record

## just ci (repository root)

- Result: passed
- Go: `go vet ./...` and `go test ./...` all passed (internal/app, runtime, rpc,
  eval, studiocore, and so on)
- UI: `pnpm typecheck` passed; `pnpm test` all 21 cases in 7 test files passed;
  `pnpm build` succeeded

## Browser smoke (http://127.0.0.1:3015, split Vite)

- The development loop was already running (backend :8787 + Vite :3015), and
  the change took effect through HMR
- Desktop-width screenshot (1440 layout simulation):
  `ui/test-results/topbar-center-1440.png`
- Quantitative measurement: the pill switcher’s center differed from the top-bar
  center by -0.01px, i.e. it was precisely centered
- The left group (menu button + avatar + status badge) and right group
  (session/to-do icons) each displayed completely on one line, with no overlap or wrapping
- Medium-width safety: the grid’s `1fr` columns use min-content as the lower
  bound; when space is tight, the middle auto column compresses first, and the
  switcher button’s built-in `max-w` + `truncate` contracts cleanly without
  overlapping side content above 768px

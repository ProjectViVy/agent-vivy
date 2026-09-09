# verification.md — Remove the 「Persona」 panel from Settings

All commands were run from the repository root `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`
(unless noted otherwise).

## Automated gates

Ran `just ci` (fmt-check → vet → test → headless-compile → ui-ci: pnpm typecheck + vitest +
vite build):

- `gofmt -l`, `go vet ./...`, and `go test ./...` all green.
- `pnpm typecheck` (tsc --noEmit) passed.
- `pnpm test`: 15 test files / 105 cases all passed (including `demo-api.test.ts`,
  `i18n/index.test.ts`).
- `pnpm build` (vite build) succeeded and generated the dist artifact.

Result: `just ci` exit code 0 (run twice: first before the parallel lane landed, and second
after `30c78b8`, revalidating the current HEAD; both green).

## Browser-path smoke test (split Vite :3015)

The development path was already running (`just run` + `cd ui; pnpm dev`), and
`http://127.0.0.1:3015` returned 200. Exercised with a Playwright script (temporary file,
deleted after verification):

```json
{
  "settingsHeading": 1,
  "tabs": ["General", "Model", "Tools", "Vivy Features", "Channels\nPreview", "Network\nPreview", "Language\nPreview", "Compaction\nPreview", "Self-evolution\nPreview", "Sandbox\nPreview"],
  "personaTabGone": true,
  "personaConfigGone": true,
  "personaPageHeading": 1,
  "identityButton": 1
}
```

- The settings-page tab list no longer contains 「Persona」, and the page no longer shows
  「Persona configuration」 text.
- The sidebar 「Persona」 link still reaches the `/persona` page (heading + IDENTITY.MD button
  present).
- No related console errors.

## Not verified

- `pnpm e2e` (the full Playwright suite) was not run; this round only manually smoked the
  affected two steps removed from `runtime.spec.ts`, and the semantics of the remaining e2e
  cases are unaffected.

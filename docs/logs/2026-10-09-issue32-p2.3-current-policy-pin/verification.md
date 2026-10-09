# Verification

## Red evidence

Before the fix, `TestCognitiveCurrentMissionBindingAdmitted` failed with `mission_revision_changed`: construction-time Mission revision 1 was checked against current revision 2 even though the resolver had already returned revision 2.

## Green evidence

- `go test ./internal/runtime ./internal/modules/diva-cognitive -count=1` — passed after atomic fail-closed wiring (`internal/runtime` 38.356s, module 0.073s), using Go 1.26.4.
- Focused `TestMissionPinnedDomain*`, Mission assignment/change race and supplied-policy resolver regressions — passed.
- `go test ./internal/app -count=1` — passed (6.308s); a temporary `ui/dist/.keep` satisfied the package's embed pattern, then was removed.
- Garden `go test ./agentapi -count=1` — passed (0.174s), including atomic Mission check/apply serialization and stale-revision rejection.
- `go vet ./internal/runtime ./internal/modules/diva-cognitive`, Garden `go vet ./agentapi`, and `git diff --check` in both worktrees — passed.
- Independent review found no remaining findings after the atomic gate fallback was changed to fail closed.

`pnpm -C ui build` did not complete: pnpm could not reach the configured `registry.npmmirror.com` package endpoints. No UI sources changed. Eino remains at `v0.9.13`; no SDK-generated contract changed.

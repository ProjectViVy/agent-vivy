# Verification record

Date: 2026-08-30. All commands ran in worktree
`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-skill-market`
(branch `feat/skill-marketplace`).

## Gate

| Command | Result |
|---|---|
| `go test ./internal/runtime/ ./internal/rpc/ ./internal/config/ ./internal/tools/ ./internal/app/ -count=1` | ok (runtime 42.5s / rpc 17.8s / config 1.6s / tools 0.4s / app 3.9s) |
| `just ci` (fmt-check, vet, go test ./..., headless-compile, ui-ci) | First fmt-check failed (gofmt alignment in app.go/control.go); after `gofmt -w`, rerun **EXIT=0, all green** |
| `pnpm typecheck` / `pnpm test` / `pnpm build` (included in ui-ci) | All 175 tests in 21 test files passed; build artifact is healthy |

New test coverage (kernel):

- `internal/runtime/marketplace_test.go`: search mapping / limit clamping / short-query
  rejection, install persistence + ListSkills visibility + reinstall conflict,
  name/slug mismatch, missing root SKILL.md, binaries, path traversal, an id Vivy
  cannot carry (rejected before download), upstream errors (`MarketplaceUpstreamError`
  carries upstream detail), invalid base URL, `VIVY_SKILLS_MARKETPLACE_URL` override,
  and built-in featured-snapshot parsing.
- `internal/runtime/skills_backend_test.go` (additional): `enabled: false` is hidden
  from Eino List/Get but visible in the control plane; SetSkillEnabled enable/disable
  round trip, stale CAS hash rejection, and rerender preserving body and description.
- `internal/config/config_test.go`: default `skills_marketplace_url`, override parsing,
  and invalid-URL rejection.

## Real-path smoke (split dev, non-embedded UI)

Environment: worktree backend `go run ./cmd/vivy` (127.0.0.1:18787,
`runtime.mock: true`, `skills_root: data/skills`,
`skills_marketplace_url: http://127.0.0.1:8899`); Vite `pnpm exec vite --port 3016`
(`VIVY_BACKEND_ADDR` points to 18787); local Python skills.sh mock
(`data/smoke_marketplace.py`, placeholder 8899 — outbound TLS is restricted in this
sandbox, so real skills.sh is unreachable; market-URL semantics mirror the HTTPS path).
Browser-tested `http://127.0.0.1:3016/skills`:

1. **The Demo banner disappeared**; tabs are Installed skills (0) / Marketplace /
   Change requests (0).
2. **Capability gating**: the Marketplace tab appears (`initialize` broadcasts
   `skills.marketplace`).
3. **Featured**: with no search, the built-in featured ranking appears (100 entries
   including find-skills 846.6k, snapshot date 2026/8/21); k/m install counts format
   correctly.
4. **Search**: entering `demo` (after debounce) hits the local mock's
   `demo-market-skill` (12.3k) and displays "1 result".
5. **Install**: click Install → the Installed tab becomes (1), and the button changes
   to "Installed"; disk contains SKILL.md + references/guide.md under
   `data/skills/demo-market-skill/`; the snapshot's README.md is skipped as expected.
6. **Details**: display description, content hash (SHA-256), attached-file count,
   clickable references/guide.md, and the SKILL.md body.
7. **Enable/disable**: toggle the switch → list and details badges change to "Disabled"
   (two locations); SKILL.md frontmatter becomes `enabled: false` (normalized rerender,
   body preserved).
8. **Change requests**: empty state "No pending Skill revisions."
   (`skills/revisions/list`).
9. The first-run wizard opens normally and can be skipped (unrelated to this iteration;
   recorded only as part of the smoke path).

The backend/Vite/mock processes were stopped after smoke (ports 18787/3016/8899
released).

## Coverage gaps (recorded honestly)

- **Non-empty rendering of pending revisions was not browser-smoked**:
  `skill_manage` staged revisions depend on an HITL proposal in a real run, and the
  mock scenario does not drive skill_manage. Unit tests cover the path
  (PrepareSkillProposal → ListPendingSkillRevisions → RPC mapping); the UI card shares
  its rendering path with the empty state, but browser-level non-empty validation was
  not performed.
- **Real skills.sh network path**: sandbox outbound TLS is restricted, so search/install
  used the local mock. The adapter and upstream contract (`/api/search`,
  `/api/download/{owner}/{repo}/{slug}` JSON shape) match the DIVA implementation and
  wiremock unit tests; public connectivity needs verification on a development machine
  (`just run` defaults to `https://skills.sh`).
- e2e (`just ui-e2e`) was not run: two existing stale specs under `UI-E2E-STALE` still
  fail (see `docs/TODO.md` §0.1; not introduced by this iteration).

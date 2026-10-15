# Verification

All commands were run from the repository root unless stated otherwise, with
`TEMP`/`TMP` inside `.workspace/tmp` (Go, esbuild and SQLite need a writable
temp directory in this environment).

## Automated

| Command | Result |
| --- | --- |
| `go test ./internal/modules/diva-cognitive -run 'TestPersona\|TestDispatcherNeverExposesInternals' -count=1` | ok (6.9s) |
| `pnpm typecheck` (`ui/`) | clean |
| `pnpm test` (`ui/`) | 91 files / 690 tests pass |
| `node scripts/check-i18n-completeness.js` | PASS en=1575 keys / 163 placeholders; zh parity; runtime copy audit clean |
| `node scripts/check-i18n-cross-face.{test.js,js}` | 8 tests pass; `PASS: 14 shared semantic units; Web and TUI en/zh projections and arguments conform` |
| `go run ./sdk/internal/cmd/source-hash plugins/vivy-persona <previous>` | new digest `d626d1aa…`, then a fixed point (`Tree(root, d626d1aa…) == d626d1aa…`) |
| `go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go` | regenerated, byte-identical (the headless artifact does not embed Module source digests) |
| `node ui/scripts/stage-ui-assembly.mjs` | `ui/src/generated/assembly.ts` regenerated; carries `d626d1aa…` for `vivy.persona.sidebar` |
| `just ci` | see below |

New tests added by this change:

- `internal/modules/diva-cognitive/actions_test.go`: `TestPersonaWriteFailureKeepsItsFixableCode`,
  `TestPersonaInitializeCapFailureIsLegible` (a real 801-character identity
  through the authority keeps `persona_cap_exceeded` and names `IDENTITY.MD`).
- `sdk/ui/src/module.test.ts`: the typed `shell.onboarding` contract, including
  two `@ts-expect-error` cases that must not compile.
- `ui/src/plugins/presentation-host.test.tsx`: the gate slot renders nothing
  when empty, renders only typed contributions in order, and keeps a throwing
  contribution inside the existing error boundary.
- `plugins/vivy-persona/ui/vivy-persona/src/view.test.tsx`: the guided walk
  submits all five documents through `persona.initialize`; a missing answer
  returns to the first empty page without calling the authority; a rejected
  write keeps every answer and shows the authority's own message.
- `plugins/vivy-persona/ui/vivy-persona/src/gate.test.tsx`: shows only for
  `uninitialized`, stays quiet for `ready`, no session, an unavailable
  authority, or a dismissed gate, and records the dismissal on skip.
- `ui/src/hooks/use-companion-avatar.test.tsx`: default, round trip, poisoned
  and unavailable storage, notify-on-change, guard totality.
- `ui/src/components/layout/CompanionAvatar.test.tsx`: `photoFailureFor`
  separates an oversized photo from something that is not a photo.

### `just ci`

Passing in this environment: `ensure-laputa` (all pinned Laputa revisions match
the source lock), `bootstrap-test` (11 pass), `fmt-check`, `ui-ci` (PNPM
install, `tsc --noEmit`, 690 tests, `vite build`, both i18n gates), `vet`,
`headless-compile`, `plugin-ci` (every plugin `go vet` + `go test`, including
`plugins/vivy-persona`), plus `internal/app`, `internal/runtime`,
`internal/rpc`, `internal/storage/*`, `internal/modules/diva-cognitive` and
`sdk/ui` inside the `test` phase.

The `test` recipe cannot go green on this machine; every failure is an
environmental capability failure, and each was reproduced **outside** this
change:

- `os.Symlink` fails with `A required privilege is not held by the client`,
  which fails `TestHashSourceTreeRejectsSymbolicLinks`
  (`sdk/internal/assembly`), `TestGenerationFailureMatrixExecutesEveryCase`,
  every `sdk/internal` pack/verify test that links `ui/node_modules` into an
  isolated build boundary (`TestPackAndInspectSealUIAssemblyIdentity`,
  `TestV1PackAndInspectProveRecipeRemoval`, `TestPackBuildsSelectedUIIntoFinalArtifact`,
  `TestPackAndInspectEveryShippedRecipe`, `TestSCXCandidateRollback…`), and
  `just diva-recall-test`
  (`TestMemoryLoopCorrectionAndDeletionInModelInput` linking
  `laputa/mentle/models`). Evidence: a plain `cmd /c mklink` in the same shell
  succeeds while the identical Go test fails alone with that message.
- `sdk/codeclient` (`TestClientAgainstRealVivyCode`) and
  `TestPackAndInspectCheckedInFullUIGateRecipe` /
  `TestVerifyEverySelectedPublicModule/full-ui-module` fail on
  `open C:\…\go-build\…-d: Access is denied`. Control: a detached
  `git worktree` of `12a16f71` (i.e. HEAD, without any change from this
  iteration) reproduces the same `sdk/codeclient` failure, so both are
  pre-existing environment/build-cache failures.

Neither group compiles or executes a file this change touches.

## Real path (split pair, real browser)

The dev pair was restarted with a disposable home
(`VIVY_USER_HOME=.workspace/onboarding-smoke`) and driven through headless
Chrome 154 over the DevTools protocol at `http://127.0.0.1:3015` (Vite proxies
`/rpc` to `127.0.0.1:8787`).

Observed, in order:

1. Fresh profile, first load: the host welcome wizard shows (`Skip wizard`).
2. Skipping the wizard makes the Module's gate appear immediately as
   `role="dialog"` labelled "Persona onboarding", on step 1 of 8, with the
   philosophy copy, `Not now`, `Next`, and the "you can finish this on the
   Persona page" hint.
3. The face step renders 12 emoji presets in a `radiogroup` plus the photo
   action; picking one stores `{"kind":"emoji","value":"🕯️"}` under
   `vivy.companion.avatar` and the header button renders it.
4. The five document steps accept real text; `Write it down` calls the
   authority once and the panel moves to "It remembers.".
5. `Start talking` closes the gate; a reload does **not** bring it back.
6. On disk, the disposable home contains real authority documents plus
   history revisions:
   `garden/persona/persona/{IDENTITY,RELATIONSHIP,USER,WORLD,REDLINE}.MD` and
   `garden/persona/persona/history/<FILE>/1.{md,diff}`; `USER.MD` carries the
   authority's `## Preferences` wrapper.
7. The header avatar popover opens the same picker (12 presets) and swapping
   the emoji updates both the stored value and the rendered glyph.
8. The photo path: a real 640×400 PNG set through the picker's file input
   produced a 256×256 WebP data URL (`data:image/webp;base64,…`, 3131 chars),
   stored it, and the header rendered an `<img>`. A non-image file showed
   "That image could not be read. Try another one." and left the stored avatar
   unchanged.
9. `Retry` on `/persona` renders the saved identity ("Your name is Xiaoman…"),
   i.e. the editor shows the committed documents rather than the onboarding.

### One pre-existing finding (not caused by this change)

A *hard load* of `/persona` (or any load that issues the page's first
`module.action.invoke` before the RPC connection is bound to the session) is
rejected with `-32009 module action is not authorized`, including for
`diva.cognitive.status`; `Retry` then succeeds. Control experiment: the same
hard load with the pre-change Module (the working tree stashed and restaged)
shows the identical error, so this is a pre-existing first-call race, not a
regression of the new gate. `/workflows` and `/notebook` do not hit it because
their first calls happen later. Recorded in `docs/TODO.md` §0.1.

Screenshots from the run are in `.workspace/tmp/shots/` (scratch, not
committed).

## Not verified

- Photo upload on a browser without a WebP encoder (the PNG fallback path) was
  not exercised; the stored-value guard accepts both.
- No mobile/viewport-specific pass beyond the responsive classes.

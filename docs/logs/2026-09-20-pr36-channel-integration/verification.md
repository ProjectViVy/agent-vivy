# CH-INT-36 integration verification

Date: 2026-09-20
Worktree: `/workspace/scratch/1b883f7e5915/vivy-channel-pr36-integration`
Branch: `feat/channel-pr36-integration`

This is Task 1 evidence only. The merge index is intentionally unresolved;
no build or product test is claimed from this gate.

## Frozen refs

```text
$ git rev-parse HEAD
c054a9b5b85a7405d071e33e3fc756114f26e2da

$ git rev-parse MERGE_HEAD
eb8fee3c0996746e93def297d700c70fb65b8540

$ git rev-parse origin/feat/channel-tier1
eb8fee3c0996746e93def297d700c70fb65b8540

$ git rev-parse origin/feat/channel-modularization
c054a9b5b85a7405d071e33e3fc756114f26e2da

$ git merge-base HEAD eb8fee3c0996746e93def297d700c70fb65b8540
fe60b1869a166490da72a07eef8f1d754caa2e3e
```

All five commands exited `0`. The source ref equals the locked source SHA and
the target ref equals the current `HEAD`.

## Source size and target descendants

```text
$ git rev-list --count fe60b1869a166490da72a07eef8f1d754caa2e3e..eb8fee3c0996746e93def297d700c70fb65b8540
69

$ git diff --name-only fe60b1869a166490da72a07eef8f1d754caa2e3e..eb8fee3c0996746e93def297d700c70fb65b8540 | wc -l | tr -d ' '
114

$ git rev-parse 0c47eb96c733a29be25cace9cd048be0c367ca10^{tree}
27d50ea95dd58fecdc0a5cdd5f1e2ede6ffd0d9a

$ git log --reverse --format='%H %s' 0c47eb96c733a29be25cace9cd048be0c367ca10..HEAD
c054a9b5b85a7405d071e33e3fc756114f26e2da docs(channel): plan PR 36 aggregate integration

$ git diff --name-status 0c47eb96c733a29be25cace9cd048be0c367ca10..HEAD
M	docs/superpowers/plans/channel-modularization/ch-p0-3-backend-omission.md
M	docs/superpowers/plans/channel-modularization/index.md
A	docs/superpowers/plans/channel-modularization/pr36-integration.md
```

The only target descendant after the planned code baseline changes planning
documents. No product-code descendant requires supervisor classification.

## Planning merge-tree reproduction

```text
$ git merge-tree --write-tree --name-only c054a9b5b85a7405d071e33e3fc756114f26e2da eb8fee3c0996746e93def297d700c70fb65b8540
```

Result: exit `1` (expected non-clean merge). The computed tree was
`aef901140178144e8e22db0cd7157216b8908b74`. The conflict-path output was:

```text
docs/COMPLETE.MD
internal/app/app.go
internal/app/channels_test.go
internal/modules/channel/binding.go
internal/rpc/control.go
internal/rpc/control_test.go
sdk/internal/assembly/conformance_results.json
sdk/internal/conformance/reproduction_test.go
```

The merge-tree diagnostics also identified the same content/modify-delete
conflicts and auto-merged the planned double-touched paths. No index entry was
staged by this command.

## Live unresolved merge

```text
$ git rev-parse --verify MERGE_HEAD
eb8fee3c0996746e93def297d700c70fb65b8540

$ git diff --name-only --diff-filter=U
docs/COMPLETE.MD
internal/app/app.go
internal/app/channels_test.go
internal/modules/channel/binding.go
internal/rpc/control.go
internal/rpc/control_test.go
sdk/internal/assembly/conformance_results.json
sdk/internal/conformance/reproduction_test.go
```

The live command exited `0` and returned exactly the eight planning paths.
The merge remains open with `MERGE_HEAD` set; no conflict file was edited or
staged during this task.

## Task 5 — UI/API semantic union

Task 5 added one combined TypeScript fixture that carries the optional
`ui_extensions` projection, both failed-delivery capabilities, a non-null
Channel health projection, and the two exact delivery RPC request names. The
first prescribed compatibility run was intentionally RED only for the missing
capability gate; the combined union assertions themselves passed:

```text
$ pnpm --dir ui exec vitest run src/components/settings/channel-store.test.ts
Test Files  1 failed (1)
Tests       1 failed | 14 passed (15)
failure: expected the negotiated-capability reader once, received 0 calls

$ pnpm --dir ui typecheck
exit 0
```

After gating both listing and redelivery on
`channel.deliveries.list` + `channel.deliveries.redeliver`, the focused store
suite passed `16/16`. A second RED proved that a stale row attempted one
redelivery after capabilities disappeared; the shared capability predicate
then made the same focused suite GREEN at `16/16` with zero unsupported calls.

The Web provenance assertion was RED before narrowing `source: string` to the
runtime's closed `ui | channel | headless` vocabulary:

```text
src/lib/api.test.ts: error TS2322: Type 'false' is not assignable to type 'true'.
```

The same closed type is exposed by `@vivy/ui-sdk`. Its fixture was also brought
up to the target's catalog/workspace API shape while retaining PR #36's health
and delivery methods.

### Final automated gates

```text
$ pnpm --dir ui test
Test Files  38 passed (38)
Tests       340 passed (340)

$ pnpm --dir ui typecheck
exit 0

$ pnpm --dir ui build
2281 modules transformed; built in 7.88s; exit 0
warning: only the known >500 kB chunk-size warning

$ node scripts/check-i18n-completeness.js
PASS: en=1417 keys / 139 placeholders; zh=1417 keys / 139 placeholders;
runtime copy audit clean.

$ node --test scripts/check-i18n-cross-face.test.js
8 passed, 0 failed

$ node scripts/check-i18n-cross-face.js
PASS: 13 shared semantic units; Web and TUI en/zh projections and arguments conform.

$ pnpm --dir ui exec vitest run --root ../sdk/ui src/module.test.ts
Test Files  1 passed (1); Tests 23 passed (23)

$ pnpm --dir ui exec tsc --project ../sdk/ui/tsconfig.json --noEmit
exit 0 (temporary SDK node_modules symlink to the validated ui/node_modules tree;
the symlink was removed immediately afterward)
```

### Split-browser limitation

No valid split-browser/network trace could be collected in this worker.
`just dev` is unavailable because this Linux executor has neither `just` nor
`powershell.exe`, so the documented equivalent split pair was attempted.
The backend started on `127.0.0.1:8787` with built-in defaults and no bot
tokens; all five compiled Channel Providers reported unconfigured/not started.
Vite's configured `0.0.0.0` launch hit the executor's
`uv_interface_addresses` restriction, while the narrow retry on
`127.0.0.1:3015` reached `VITE ready` successfully.

The only available browser is an isolated cloud browser. Its direct navigation
to `http://127.0.0.1:3015` failed with `net::ERR_BLOCKED_BY_CLIENT`, so it cannot
reach this executor's loopback. Both split processes were stopped afterward
and ports 3015/8787 were confirmed closed. No screenshot, curl response, or
unit test is presented as browser acceptance evidence. The controller must
perform all five live checks from the Task 5 brief and append the actual RPC
network trace before final acceptance.

Controller retry (2026-09-20 UTC): the same split pair was started manually
again (`go run ./cmd/vivy` on `127.0.0.1:8787` and Vite on
`127.0.0.1:3015`). The cloud browser reached the browser binding and attempted
the exact local URL once, but navigation again returned
`net::ERR_BLOCKED_BY_CLIENT`. The backend and Vite sessions were stopped after
the failed navigation. Consequently the five live checks remain explicitly
unclaimed: inspect/get RPCs, health/configured/running rendering, absent-
capability suppression, empty delivery-list rendering, and ordinary
chat/run rendering. No RPC network trace was fabricated.

## Task 6 evidence correction — canonical Provider fixed points

The earlier provisional Provider values (`09a1cb...`, `6a8bbc...`,
`559e321...`, `1bcad6...`, `8ebb4a...`) were raw source-tree hashes computed
with an empty declared digest. They were rejected as release evidence because
the five Provider descriptors/manifests are self-describing and retain these
canonical fixed points:

```text
dingtalk  2bb377560b27c5f24db1535dc2eaf86148edbe9a9a2977970d86d7a4ff05fff8
discord   ee1606ec75a2b4d6ac4ce5b0ed8cc0acdd2a6668d5f1c4051421b44ce2a092d7
feishu    c636a1ea060cb5bbb5ec88b41ed35337da708d439a9c54fea5b2e32d3428969a
qq        15d8cd19fdc01e0e5c5b21f6a623c882726085eccdc1d9d18eca27aa250d3ef8
telegram  acabc17a459b99a928dac3d4284b178137def8dbd3c022ed318b417b1b5ca00d
```

The raw values allowed the producer's declaration check to pass, but did not
match the selected Provider descriptors. Consequently Pack/Inspect produced
no `std/channel@v1` conformance results for the real source-bound identity.
The correction changed only the five Channel Provider entries in
`sdk/internal/conformance/reproduction_test.go` and the matching five entries
in `sdk/internal/assembly/conformance_results.json`; internal entries and
unrelated suites were left unchanged. The prior RED pack/parity observation
is intentionally retained below.

Fresh evidence, using `GOFLAGS=-buildvcs=false` with the repository Go 1.26.4
toolchain:

```text
go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
ok  agent-vivy/sdk/internal/conformance  67.862s
go test ./sdk/internal/conformance -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)' -count=1
ok  agent-vivy/sdk/internal/conformance  0.014s
go test ./sdk/internal/assembly -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)' -count=1
ok  agent-vivy/sdk/internal/assembly  0.028s
```

The requested temporary default pack completed with generation
`9d4a3db9dd9cc358c786b0a537340447ac1fd223a1b09c258aab7321acf02927`.
A small jq check over `generation.json` confirmed five unique
`std/channel@v1` providers and exact fixed-point hashes; comparing the packed
manifest from `inspect-artifact` with `generation.json` also passed:

```text
jq fixed-point parity: PASS (5 unique std/channel@v1 providers)
inspect-artifact manifest parity: PASS
```

The temporary pack output was created outside the repository. No commit was
made, and only the two evidence files were staged for this correction.

## Task 6 — durable records and source evidence

The three Task 6 merge conflicts were resolved in the open ancestry-preserving
merge without a commit:

- `docs/COMPLETE.MD` is a union of the current provider-registry and P0-1/P0-2
  completion rows with PR #36's Channel completion rows.
- `sdk/internal/conformance/reproduction_test.go` keeps
  `releaseSuiteCases(internalDigest string)`, the five Channel Provider cases,
  and their executable commands; no hard-coded `internalSHA` remains.
- `sdk/internal/assembly/conformance_results.json` is syntactically valid and
  contains the provisional union before producer verification.

Source identity was computed with the repository tool:

```text
$ go run ./sdk/internal/cmd/source-hash internal ""
cefe64bcd1d80c3221702cada65781e902be1904522864273768faf8cfb43f6a

$ go run ./sdk/internal/cmd/source-hash plugins/dingtalk ""
09a1cb7d6da0a84d696f5d10d5567a11a1435bcd1d0dd3722eec08aa6e1b12a7
$ go run ./sdk/internal/cmd/source-hash plugins/discord ""
6a8bbc67f943733eb7661445e47680ce4386ddc2d5eff0c328826eded03774de
$ go run ./sdk/internal/cmd/source-hash plugins/feishu ""
559e3214b7d5c6bb160a29d2a2f5bb581d0f8746475f90ef3a19ac11e6a6de1b
$ go run ./sdk/internal/cmd/source-hash plugins/qq ""
1bcad6da5bdc47d2c0f44ca40e001a5bf70237ca48cbe3e726814cde4ab905a6
$ go run ./sdk/internal/cmd/source-hash plugins/telegram ""
8ebb4aa2d93a1902f10ca1e14e95ae26d6e90536c96037f100fe1b98bf137087
```

The JSON source entries were rotated only for these changed source roots. The
JSON parser check passed. The prescribed producer test was run with the
repository Go 1.26.4 toolchain, but could not reach digest comparison because
the merged tree's existing `sdk/internal/assembly/channel_capability_test.go`
does not compile: it calls the absent `apphost.BindChannels` symbol at line
45. The same compile blocker affects the assembly-focused command below; no
passing conformance evidence was recorded from either failure.

```text
$ go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
FAIL: sdk/internal/assembly/channel_capability_test.go:45:25:
      undefined: apphost.BindChannels

$ go test ./sdk/internal/conformance -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)' -count=1
ok   agent-vivy/sdk/internal/conformance  0.018s

$ go test ./sdk/internal/assembly -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)' -count=1
FAIL: sdk/internal/assembly/channel_capability_test.go:45:25:
      undefined: apphost.BindChannels
```

No PostgreSQL, browser, or CI evidence was fabricated. The open merge has no
unresolved index paths after staging the three resolutions; final conformance
acceptance remains blocked on the pre-existing Task 5 assembly test symbol.

### Task 6 blocker resolution and final evidence rerun

The assembly compile failure was resolved as a P0-2 test-boundary adaptation:
`sdk/internal/assembly/channel_capability_test.go` now calls
`internal/modules/channel.BindProviders` with `channelcontract.Config{}`.
This is test-only; no `internal/app` compatibility wrapper or production
boundary was added.

Using `GOFLAGS=-buildvcs=false` for this isolated worktree, the exact gates
that were previously blocked now pass:

```text
$ go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1
ok   agent-vivy/sdk/internal/conformance  69.223s
$ go test ./sdk/internal/conformance -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)' -count=1
ok   agent-vivy/sdk/internal/conformance  0.018s
$ go test ./sdk/internal/assembly -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)' -count=1
ok   agent-vivy/sdk/internal/assembly  0.030s
$ python3 -m json.tool sdk/internal/assembly/conformance_results.json
valid JSON
```

The flag only disables executor-local VCS stamping for nested Go commands; no
repository behavior or checked-in evidence semantics were changed.

## Task 7 local closeout gates

The final local pass completed after the Task 6 fixed-point correction:

```text
gofmt: clean
go vet <all main-module packages except agent-vivy/internal/workflow>: PASS
go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui: PASS
go test -timeout 20m <all main-module packages except agent-vivy/internal/workflow>: PASS
independent plugins/* and faces/* go test ./...: PASS
pnpm --dir ui test: PASS (38 files, 340 tests)
pnpm --dir ui typecheck: PASS
pnpm --dir ui build: PASS (known >500 kB chunk warning only)
SDK UI tests/typecheck: PASS (typecheck used repository ui dependency typeRoots)
```

The literal repository gate was attempted and is unavailable in this
executor:

```text
$ just ci
/bin/bash: just: command not found
```

No undocumented local substitute is being presented as `just ci`. Final
external acceptance remains: a GitHub Actions run that executes the literal
gate and browser smoke, a live PostgreSQL DSN/service for dual-backend tests,
and a browser-visible split trace covering the five Task 5 checks. The local
branch is ready for review but not yet accepted or pushed.

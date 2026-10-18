# Verification

Toolchain: Windows 10.0.26100 / Git Bash, Go 1.26.4 windows/amd64, Node
25.9.0 ambient (24.14.1 via nvm for the UI gate), pnpm 10.33.0, just 1.49.0,
git 2.53.0.windows.2. WSL2 Debian (kernel 6.18.33.2) with Go 1.26.4, Node
24.21.0 (nvm), pnpm 10.33.0, just 1.40.0, Docker 26.1.5+dfsg1, and a local
PostgreSQL 16.15 at `127.0.0.1:55432`. WSL is capped at 8 vCPU, 8 GB memory,
and 4 GB swap. Laputa checkout: `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`,
clean.

WSL gate commands prepend `$HOME/.nvm/versions/node/v24.21.0/bin`,
`$HOME/.local/go/bin`, and `$HOME/.local/bin`. A plain `bash -lc` can resolve
Debian's Node 20 from `/usr/bin/node`, which fails the Node 22+ requirement.
Proxy and registry overrides for the runs below:
`GOPROXY=https://mirrors.aliyun.com/goproxy/,direct` and
`npm_config_registry=https://registry.yarnpkg.com`.

## Windows checks

- `just check-fast`: passed after the handshake-test change. `fmt-check`
  reported 1220 formatted Go files; `go vet ./...` and `go build ./...`
  completed without errors.
- `node --test scripts/ensure-laputa.test.mjs scripts/dependency-closure.test.mjs`:
  11/11 passed.
- `just plugin-ci`: 22 modules passed.
- `just diva-recall-test`: passed against the SDK-generated DIVA assembly.
- `just headless-compile`: passed for `cmd/vivy`, `cmd/vivy-code`, and `ui`.
- `go build ./sdk/tui/face && go test ./sdk/tui/face -count=1`: passed after
  the `rpcMode.sessionID` race fix described under the Linux race lane.
- `go test ./internal/app -run '^TestMemoryLoopEffectHandshake' -count=1 -v`:
  SKIP with `process bypasses sharing locks via backup semantics; run the
  owner test non-elevated`, then PASS. Classification: Windows owner-test /
  elevated-shell interaction, not a generation-contract or artifact defect.
  An elevated process can read through the share-mode-0 handle because Go's
  read open uses `FILE_FLAG_BACKUP_SEMANTICS`; the test now skips on that
  observation, and non-elevated CI keeps the original coverage.
- Full Windows `just ci` / `just backend-ci` was not rerun locally in this
  round; the Windows CI lanes own it. The ambient Node 25 still hits the known
  `UI-VITEST-NODE25` issue.

## Linux main development line (WSL)

Workspace: `~/work/agent-vivy` on the Linux filesystem, sibling
`~/work/laputa` at the lock commit.

- `just doctor`: 28 ok, 1 warning, 0 failures. The warning is the intentional
  process-local `GOPROXY` differing from `go env GOPROXY`; recipes never write
  `go env -w`.
- `node scripts/memory-loop.mjs`: passed. It tested 106 ordinary packages,
  packed `recipes/diva.vivy.yml`, inspected the artifact, ran the SDK
  memory-loop tests, and ran 99 app sources against the generated DIVA
  overlay. The app pass must execute with cwd `internal/app`; from the repo
  root Go treated the bare source filenames as import paths.
- `just ci`: passed. `fmt-check` 1220 files; UI 92 test files / 692 tests;
  `go vet ./...`; `go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...`;
  the memory-loop second full pass; `diva-recall`; headless compile; plugin-ci
  22 modules.
- `just test-postgres` with
  `VIVY_POSTGRES_TEST_DSN=postgres://vivy:vivy@127.0.0.1:55432/vivy?sslmode=disable`:
  `ok agent-vivy/internal/storage/postgres 132.466s`. This is a real local
  PostgreSQL 16.15 server; no skipped run was counted as a pass.
- `just race-ci`: executed; **not passed**. Two data races were reported:
  1. First-party, fixed here: `sdk/tui/face/rpc.go` `admitSettledFollowUp`
     read `m.sessionID` without the mutex while `dispatch` wrote it under
     the mutex, so `TestCodeFaceRPCModeGoldenTranscript` failed. The
     goroutine now snapshots the session id with `snapshot()` before
     polling. `go test -race ./internal/app -run '^TestCodeFaceRPCMode'
     -count=3` passed afterward (7.639s).
  2. Pre-existing upstream: `internal/runtime`
     `TestServiceBashTraversalRefusedWithoutFailingRun` reproduced
     `EINO-TOOLSNODE-ERR-RACE` in pinned
     `cloudwego/eino@v0.9.13/compose/tool_node.go:1253`, where the enhanced
     converter writes its function-scope `err` from parallel stream
     goroutines. `PG4-EINO-TOOLS-RACE` tracks the same defect. No Eino
     orchestration was changed and the pin was not upgraded, so the lane
     remains red and is not counted as passed. Eino v0.9.21 contains the
     local `convertErr` form of the fix; adopting it is a separate
     authorized dependency change.
  All other `./internal/...` packages passed under race, including
  `internal/rpc` (129.804s), `internal/storage/sqlite` (186.958s), and
  `internal/runtime/compaction`.

## Docker acceptance

Candidate: `5715950ddfc87e05cf60e3d2a0ca54983bef05eb` with 42 dirty entries,
copied into an empty `~/vivy-acceptance` parent next to a Laputa checkout at
`4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24` (clean). Tracked diff digest
`4efb986d4f19f8447c86a1a25ce4dc6d52f8137f06f52b0e26ead43aff93ceb9`; copied
tree digest `96f30a306c3261000fc05b106dbdbfa4253358c33ecbf4ea8b0447d6cdb68b73`.
The copy was taken before this identity record was written; the later edit is
documentation only and does not change the image inputs.

Two runs, same image:

1. `docker build --network=none --build-context laputa=... .` with no
   registry, proxy, or DNS available to any build step. `pnpm install`
   reused 409/409 packages with 0 downloads, `go mod download` completed
   from the seeded module cache in 0.4s, and `pnpm build` plus `go build`
   completed.
2. `node scripts/docker-build.mjs --tag vivy:acceptance --smoke` — the
   documented entry, run with warm caches. It verified the Laputa pin, the
   local-replace manifests, and the UI `file:` dependency, rebuilt the same
   image, and passed the tmpfs `/data` smoke.

- image
  `sha256:ff2a1f2496be2c7138c3791ac37b0ce7d1c8cde31bc2826c849dc92a9ce17780`,
  created 2026-10-17T20:56:42+08:00, 180,313,796 bytes (about 172 MiB)
- smoke: `docker-build: smoke passed; container healthy on an empty /data`
- the container logged `vivy starting` on `0.0.0.0:8787`; stopping it logged
  `vivy shutting down`

The caches were seeded from the host into the BuildKit cache mounts
(corepack, pnpm store v10, Go module cache) with a scratch seed Dockerfile.
On this host WSL DNS returns Clash fake-IP addresses in `198.18.0.0/16` and
the Windows proxy listens only on `127.0.0.1`, so containers can reach
neither. This proves the Dockerfile inputs are complete and the image starts
without any registry; it does not prove the public mirrors are reachable
from this host. A cold-cache networked rebuild remains the release-path
check.

`DEV-ENV-DOCKER-ACCEPTANCE` is closed by this evidence.

## Not run

- Live provider smoke: no provider key was used.
- Full browser E2E: only the split dev origin and the UI unit/typecheck gates
  were exercised in this round.
- Native packaged `.exe` acceptance: the Windows split loop was exercised in
  the earlier cut, but no new packaged binary was cut for this verification.
- Full Windows backend/UI/browser lanes: owned by CI; local Windows evidence
  is listed above.

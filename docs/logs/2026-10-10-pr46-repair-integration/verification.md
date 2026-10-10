# Verification

## Final source checkpoint

The product source checkpoint is `a44bb3f9`; the final documentation-only commit does not change these source identities or artifact generations.

- Final `just ci`: PASS, exit 0. Executed with Go 1.26.4, `GOMAXPROCS=2`, `GOFLAGS=-p=2`, `CI=true`, the isolated live PostgreSQL 17 DSN, and the frozen pnpm registry override below. All required bootstrap, formatting, UI, cross-face localization, vet, Go, DIVA recall, headless compile and independent plugin gates completed.
- UI: 78 test files  / 601 tests PASS; typecheck/build, 8 cross-face tests and 14 shared semantic-unit checks PASS.
- Main Go gate: 81 package results PASS. App 677.541s, runtime 44.799s, RPC 14.939s, SDK internal 621.878s, SDK assembly 9.777s, and executed source-bound conformance producer 76.377s. The source producer recomputes the digests, executes owning Provider/Host suites and byte-compares canonical evidence. No evidence pass flags were edited.
- The six original recall test groups all PASS, zero skips, through the official SDK-generated `recipes/diva.vivy.yml` overlay. The unchanged default recipe deliberately omits native recall; a default-inventory regression verifies that selection. 21 independent plugin/face module vet/test groups PASS, including QQ.
- Final internal SHA256: `eba914561ec043b74bd56e00236fc7a48df2e20dbc8956fca39cf033288a94fe`. QQ SHA256: `ede386c84e08e19c43b161a535e8a1b70cf7eef98dafe1b725f1e7025c19c7ed`. Both were recomputed successfully after CI. QQ module declarations and the independent reproduction table match. `go generate ./internal/generated/assembly` reproduced the checked-in declared headless form byte-for-byte.

## Clean checkout and actual artifacts

A detached clean checkout at `a44bb3f9` used a pristine bootstrap-cloned Laputa sibling at exact locked SHA `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`. Bootstrap, headless cmd/vivy + cmd/vivy-code + SDK + runtime compilation, frozen UI build, and embedded App + SDK internal + UI compilation PASS. The tracked checkout remained clean before and after; no developer sibling content was substituted.

`go run ./sdk pack --recipe recipes/<minimal|default>.vivy.yml --output <scratch>` PASS. For each artifact, the pack result, `go run ./sdk inspect-artifact <scratch>`, and the actual binary's `--inspect-generation` agree exactly:

| Recipe | Modules | Final GenerationID |
| --- | ---: | --- |
| Minimal | 7 | `c76e000504d88da595abeb4ddab1d8b1407e3bba79624e1689d74e034fed9e23` |
| Default | 37 | `bcfff9a07a0cb5db9761022c3891d86818a6fd282cb25ee45436f64beae71b01` |

Both artifacts carry the final internal source identity and matching dependency locks. Public Laputa module Origin.Hash and all six checksums match the source lock. SDK dependency inspection contains no concrete optional cognitive/memory/mask implementation imports. SDK hello-fs verification also passed earlier in integration.

## Product flow, durability and races

- Final browser command: `PLAYWRIGHT_CHROMIUM=/usr/bin/chromium pnpm --dir ui exec playwright test --config playwright.queue.config.ts`: PASS, 1 test, 23.1s. Real headless backend:8787, split Vite:3015, isolated SQLite and loopback model:8797. A busy high-thinking PNG survives reload, manual recall and resend, and reaches the actual second model request; queue ends empty. No live provider call or production journal was used.
- Integrated race command: `go test -race ./internal/runtime ./internal/logging ./sdk/tui/live ./sdk/tui/stream ./sdk/tui/view -run 'Test(Queue|ConcurrentQueue|FailedAdmissionQueue|SealedClear|RealParkedCancellation|ParkedCancellation|PendingCancellation|CancelCommitsQueue|Steer|ClearQueue|Observer|CacheWarm|Diagnostics|.*Restore|.*Returned)' -count=1`: PASS, runtime 50.202s, logging 1.028s, TUI live 1.124s/stream1.019s/view1.234s.
- Final queue boundary change: four focused tests PASS (0.287s) and their race selection PASS (5.152s). The new test first reproduced an acknowledged steer needing a 65,538-byte restoration marker against 65,536. Both future track fields are now reserved before ACK; rejection leaves memory and Journal unchanged, accepted persisted track remains steer, and a near-limit actual missing-checkpoint fallback completes durable admission. Independent read-only review found no blocker.
- Real PostgreSQL 17 suite PASS (23.447s) before final control cleanup; the earlier integrated gate executed the final unchanged storage suite with the live DSN and PASS (22.511s). Final CI reused that valid cached result; SQLite also passed. Queue controls cleanup, transactional ownership and rollback parity have actual PostgreSQL evidence.
- Independent final recovery review confirmed parked cancellation retry, sealed-carrier removal/recall, replay and commit failures, first-writer interaction decisions, complete DTO restoration and session cleanup. The individual M1–M6 and queue-editor records contain focused provider-failure, RED-to-GREEN and cross-platform evidence.

## Failures resolved during integration

Exact old-pin clean sibling compilation reproduced all five unresolved Laputa symbols. Original local attachment FIFO and text-only recall were independently RED. Earlier incomplete CI attempts exposed unclassified restoration localization keys, scratch-installed Go fixture traversal during vet, recipe-inaccurate recall guards and a QQ mock factory/authentication race. Each cause was fixed with focused evidence; none of those attempts is counted as passing. The in-flight CI preceding the final marker boundary change was stopped before source freeze. Only the final complete run above is the release gate.

## Environment, cleanup and limits

Go 1.26.4, PowerShell 7.6 and just 1.42 used excluded task scratch because this cloud image's `go` is an unrelated CLI. Frozen pnpm installs used `PNPM_CONFIG_REGISTRY=https://registry.npmjs.org` because the repository npmmirror endpoint is blocked; dependency lockfiles were unchanged. A scratch-only nested module excluded the task toolchain from Go `./...`.

Task worktrees, bootstrap-cloned validation siblings, packaged scratch binaries, isolated browser state and PostgreSQL container were removed after verification; focused branches were retained. No push or deployment was performed.

Broad `go mod verify` encounters missing ziphash for existing local v0.0.0 replacements and is not claimed as passing. Scripted same-prefix write/read evidence validates accounting, not live monetary savings; `CACHE-WARM-NET-BENEFIT` remains open before any default-on proposal. Windows diagnostics were cross-compiled but not runtime-tested. Accepted full queue records must fit the configured durable event ceiling (64 KiB default); oversized requests are explicitly rejected before ACK.

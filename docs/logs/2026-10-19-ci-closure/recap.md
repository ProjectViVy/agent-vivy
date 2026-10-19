# S5: Adopt the published Recap recovery cancellation fix

## Outcome and scope

Vivy now pins published Laputa commit
`0d787277f521dee0d88d1beb522c34681e3ba57e` consistently in
`laputa-source.lock.json`, all three root Go pseudo-versions, and their module
checksums. The pseudo-version is `v0.0.0-20261019182534-0d787277f521`:
`git show --format=fuller --stat 0d787277f521dee0d88d1beb522c34681e3ba57e`
confirmed the commit timestamp of 2026-10-19 18:25:34 UTC.
`git ls-remote origin refs/heads/feat/issue83-laputa-closure` returned this exact
commit from the canonical remote. `go mod download -json` for each of
`github.com/ProjectViVy/laputa/{garden,laputa,mentle}` at that version succeeded;
its reported `Sum`, `GoModSum`, and origin hash supplied the six `go.sum` entries.
No checksum was inferred from the old version.

The existing independent real-Go harness needed no code or assertion adaptation.
Its README now describes the fixed pin instead of promising an intentional
failure. Existing native regressions and Python comparisons are unchanged; no
skip, expected failure, owner-guard weakening, error-cause removal, diagnostic
production overlay, or sleep was introduced.

The published delta from `0e0027c5` is only a three-line recovery error guard,
its native regression, and its upstream evidence document. The root cause was
`recapChain` marking the owner unknown on every `recoverRecap` error, even when
cancellation happened before replacement and the underlying context error proved
the attempt safe to retry. At the new pin,
`laputa/actmem/recap_history.go:52-57` returns that safe cancellation without
poisoning the owner. After-replacement uncertainty and unrelated I/O failures
still take the existing unknown-commit path. The adopted upstream test
`TestRecapRecoveryCancellationKeepsReplacementBoundary` covers all three cases,
including preservation of the I/O error cause. No Laputa source was edited.

Eino capability check: dependency adoption and existing test execution only; no
LLM loop, ADK, context-admission policy, or orchestration implementation changed.

## Isolation and environment

- Linux amd64; Python 3.10.12; `go version go1.26.4 linux/amd64`.
- Vivy baseline: `e46d332362608062b44509810628e66bce97a712`.
- Lane created with the owner-specified command:
  `node /home/ubuntu/memory-write-model/agent-vivy/scripts/lane-new.mjs ci-repair-recap --base e46d332362608062b44509810628e66bce97a712`.
- Checkout: `/home/ubuntu/memory-write-model/agent-vivy/.worktrees/ci-repair-recap/agent-vivy`.
- Only this lane's private sibling `../laputa` was advanced, with the unchanged
  official `node scripts/ensure-laputa.mjs`; it reported the exact new pin ready.
  `git status --short` in that sibling was empty after verification.
- Every test shell inherited the following process-local environment, including
  subprocess Git fixture commits. No Git configuration or remote was edited:

```sh
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME='大湿' GIT_AUTHOR_EMAIL=1136523089@qq.com
export GIT_COMMITTER_NAME='大湿' GIT_COMMITTER_EMAIL=1136523089@qq.com
export GOTOOLCHAIN=go1.26.4 GOPROXY=https://proxy.golang.org,direct
```

All mutable stores and worker binaries were disposable test roots. The original
checkout, its sibling Laputa, production `data/`, journals, and `~/.vivy` were not
used. No server, UI build, or UI asset copy was needed by these focused packages.
No delivery commit, push, PR, merge, or deployment was performed.

## Red evidence before changing pins

Commands below ran from the lane checkout at the old lock
`0e0027c5f60ad67614d3613c2d5e21d567c4b412`:

```sh
python3 scripts/memory_model.py --replay scripts/testdata/memory-model/recap-cancel.json
python3 scripts/memory_model.py --target recap --seed 17 --cases 1 --steps 100 --race
```

Both exited 1 with a model/native mismatch, not a setup failure:

- Curated seven-operation replay: seed 0, zero-based step 2,
  expected `{'error': 'context_canceled'}`, actual
  `{'error': 'recap_commit_unknown'}`.
- Generated race-instrumented trace: seed 17, zero-based step 209, the same
  expected/actual mismatch. The complete trace has 213 operations.
- Synthetic failure receipts were saved under ignored
  `.workspace/memory-model/recap-0.json` and `recap-17.json`. They retain the old
  source pin and were not added to the patch. Replay creates fresh store roots.

## Green evidence after adopting the published pin

All commands in this section exited 0. Root-relative commands use the lane
checkout unless another module is named.

| Command | Observed result |
| --- | --- |
| `node scripts/ensure-laputa.mjs` | Exact published pin ready through the unchanged canonical-origin and clean-source checks. |
| `node --test scripts/dependency-closure.test.mjs scripts/ensure-laputa.test.mjs` | 11 passed, 0 failed, 0 skipped; includes rejecting unrelated origins, dirty source, wrong module identity, and revision/toolchain mismatches. |
| `python3 -m unittest discover -s scripts -p test_memory_model.py -v` | 15 passed; includes strict protocol/receipt failures, source guard, and both cancellation-boundary reference contracts. |
| `python3 -m py_compile scripts/memory_model.py scripts/test_memory_model.py` | Passed. Generated untracked `scripts/__pycache__/` was removed after verification. |
| `python3 scripts/memory_model.py --target all --seed 17 --cases 20 --steps 100 --race` | All 40 real-Go differential traces passed: BML seeds 17-36, 204 operations each (4,080 total); Recap seeds 17-36, 213 operations each (4,260 total). 8,340 operations overall; no race report. |
| `python3 scripts/memory_model.py --replay scripts/testdata/memory-model/recap-cancel.json --race` | Seven-operation curated cancellation trace passed on a fresh root. |
| `python3 scripts/memory_model.py --replay .workspace/memory-model/recap-17.json --race` | Previously failing 213-operation seed-17 receipt passed on a fresh root against the new pin. |
| `python3 scripts/memory_model.py --target all --go-regressions --race` | BML native replay/tombstone-after-reopen regression passed; Recap native cancellation regression passed both `recovery_before_replace` and `recovery_after_replace` subtests. |
| In `bml`: `go test -mod=readonly -p=1 ./...` | Full BML module passed. |
| BML ownership/replay command below (in `bml`) | Selected ownership/replay regressions passed three repetitions. |
| In private `../laputa/laputa`: `go test -race -mod=readonly -p=1 -count=3 ./actmem` | ACTMEM package passed three repetitions, including the newly adopted upstream boundary/I/O regression. |
| `go test -race -mod=readonly -p=1 -count=1 ./internal/modules/diva-cognitive` | Full cognition module package passed (40 top-level test functions), including Recap CAS, authorization, exact Unicode/JSON, owner lifecycle, reconcile, and capture receipt compatibility. |
| `go vet -mod=readonly -p=1 ./internal/modules/diva-cognitive` | Passed. |
| In private `../laputa/garden`: `go test -race -mod=readonly -p=1 -count=1 ./agentapi ./internal/activity ./internal/recall ./internal/runtimecore` | All four selected compatibility packages passed. |
| In private `../laputa/garden`: `go vet -mod=readonly -p=1 ./agentapi ./internal/activity ./internal/recall ./internal/runtimecore` | Passed. |
| `gofmt -l scripts/testdata/memory-model/bml_test.go scripts/testdata/memory-model/recap_test.go` | No unformatted files. |

Exact selected BML race command, run in `bml`:

```sh
go test -race -mod=readonly -p=1 -run 'Test(Bound|.*Replay|.*Owner)' -count=3 ./...
```

Overlay vet also passed for both actual packages using the existing bridge as a
virtual test-only file. Exact reproduction from the lane checkout:

```sh
python3 - <<'PY'
import json
from pathlib import Path
import subprocess
import tempfile
root = Path.cwd()
with tempfile.TemporaryDirectory(prefix='ci-recap-vet-') as directory:
    for target in ('bml', 'recap'):
        module = root / 'bml' if target == 'bml' else root.parent / 'laputa' / 'laputa'
        package = module if target == 'bml' else module / 'actmem'
        overlay = Path(directory) / f'{target}-overlay.json'
        overlay.write_text(json.dumps({'Replace': {str(package / 'memory_model_test.go'): str(root / 'scripts' / 'testdata' / 'memory-model' / f'{target}_test.go')}}))
        command = ['go', 'vet', '-mod=readonly', '-p=1', '-overlay', str(overlay), '.' if target == 'bml' else './actmem']
        print(f'{module}: {command}', flush=True)
        subprocess.run(command, cwd=module, check=True)
        print(f'PASS overlay vet {target}', flush=True)
PY
```

One verification command was corrected, not hidden. In `../laputa/garden`:

```sh
go test -race -mod=readonly -p=1 -count=1 ./internal/authority ./internal/activity ./internal/recall ./internal/runtimecore ./internal/agentapi
```

This requested nonexistent `./internal/authority` and `./internal/agentapi` alongside
activity/recall/runtimecore. It exited 1 with two `directory not found` setup
errors; the other three packages passed. Inspection located the public
`./agentapi` package; the corrected four-package command above then passed.
No source or assertion was changed to address that command error.

## Delivery and untested boundaries

The patch owns only root dependency metadata, the harness README correction, and
this log. No additional runtime changes or regression test duplication were
needed because the baseline harness already reproduced the exact failure.

Native Windows qualification, aggregate `just ci`, whole-product race and real
PostgreSQL acceptance, independent integration review, and unrelated full
evolution acceptance are not claimed here. S6 owns the aggregate/platform gates.
These results do not establish power-loss durability, uncommitted-write crash
behavior, or Windows filesystem barriers. UI testing was neither needed nor run.

Patch export is uncommitted at
`/home/ubuntu/memory-write-model/ci-closure-results/recap.patch` and contains this
log via explicit `git add -N -- docs/logs/2026-10-19-ci-closure/recap.md` before
`git diff --binary`. `git diff --check` passed. The integration owner must apply
all Story patches and run the common gates before any broader acceptance claim.

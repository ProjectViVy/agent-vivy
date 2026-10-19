# Verification

Go 1.26.4 linux/amd64. Every test/vet/module-list command used the unchanged
`/tmp/issue58-localonly-runner.py`: offline dependencies, credential-name stripping,
loopback-only fixtures and a non-forwarding deny proxy. All recorded zero external
requests and zero forwarding. No live model, tenant database or credentials were
used. The exact unchanged module dependencies were prepared separately by root
through the official proxy/sumdb; no dependency file changed.

| Final working-source gate | Tests pass/fail/skip | Package pass/fail | Seconds |
| --- | --- | --- | --- |
| Native SQLite 1.56.0 normal | 155/0/0 | 1/0 | 1.424 |
| Native SQLite 1.56.0 race | 155/0/0 | 1/0 | 7.354 |
| Root MVS SQLite 1.59.0 normal | 155/0/0 | 1/0 | 1.730 |
| Native vet | n/a | exit 0 | 0.265 |

Counts include Go parent tests/subtests. No race diagnostic occurred. Gofmt and
diff whitespace checks pass. Independent module-list records confirm each SQLite
version. Final Go bytes are bound to the committed checkpoint by host source
hashes; runner metadata names the baseline HEAD because execution preceded commit.

Commands, each prefixed by the runner and its module directory/log label:

```text
# from bml/
go test -json -p=1 ./... -count=1 -timeout=4m
go test -json -race -p=1 ./... -count=1 -timeout=5m
go vet -p=1 ./...
go list -m -json modernc.org/sqlite
# from repository root, same native package tests under root MVS
go test -json -mod=readonly -p=1 github.com/ProjectViVy/agent-vivy/bml -count=1 -timeout=4m
go list -m -json modernc.org/sqlite
```

All raw attempts are retained in `.superpowers/sdd/S5-bml-native`, including:

- Initial offline setup failure (missing exact SQLite 1.56 cache), followed by
  root's separate official dependency preparation; this is not semantic RED.
- Missing API declaration compile RED, then an explicit unavailable API shell
  semantic RED before implementation; the shell is absent from the final source.
- Actual fresh/real-Home v1 upgrade RED then GREEN. Temporary external test overlay
  only excluded the not-yet-declared new bound tests from this schema step.
- Whole-module intermediate failures in five old schema-version expectations;
  those expectations now describe fresh v2, while explicit released-v1 fixtures
  retain their original version and read-only/rollback assertions.
- Backup mutation, bound identity rewriting, owned-row GC, deleted-target CAS,
  foreign suppression, malformed native schema, raw enum and read-only Restore
  REDs followed by the complete final GREEN runs. Rejected request/source changes,
  exact scope/destination and invalid UTF-8 are also covered.
- Same-operation replay and distinct operations across two Store handles, one-winner
  record CAS, receipt-insert failure, real deferred-constraint Commit failure,
  staged-restore upgrade failure, tombstone/reopen and L1-refresh retry evidence.

The source pass is by the author; independent native review/publication is root's
next gate. Actual Windows, root whole-product CI, App/Garden adapter conformance,
actual worker/LIFE ContextHost isolation, UI and installed-product acceptance were
NOT RUN by this lane. Root owns those gates. No consumer activation is claimed.

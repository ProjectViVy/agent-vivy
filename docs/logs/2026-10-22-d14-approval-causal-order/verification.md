# Verification

The clean isolated lane fast-forwarded without reset to frozen candidate
`1b8da8cca150c7a4a7734c6e480fa41e24b230cc`. Its own sibling Laputa remains pinned
at `9acc999301d5d575da2cea902a1c6d213763f381`.

RED: the independent native full-CI log
`.workspace/issue91-native/agent-vivy/.workspace/verification/final-ci.log`
in the owner's root checkout recorded `TestServiceApprovalApproveFlow` failing
at the old line 269: event 5 was `model.request`, expected `tool.started`.
The observed prefix already contained the approval, policy events, claim and
completed operation. Read-only source inspection confirmed that completed
operation and its Journal event commit atomically before returning to Eino;
tool.started/finished are reconstructed later by the consumer.

GREEN, native Go with `GOFLAGS=-p=2` and checkout-local temporary directory:

```powershell
go test -tags=vivy_headless ./internal/runtime -run '^TestServiceApprovalApproveFlow$' -count=1 -timeout=1m
git diff --check
```

The first corrected run passed in 1.662s. After final assertion cleanup, the
final focused run passed in 1.574s; diff check passed. This compiles the runtime
test package and exercises real Service/SQLite approval and native Eino resume.
The temporary Go directory was removed afterward.

Per the integration owner's slot/scope instruction, no broad suite, full CI,
race, browser, live model, or new platform gate was launched. The parent owns
combined-candidate re-audits and final acceptance; this log does not supersede
the earlier full-CI failure or claim that all its unrelated failures are fixed.

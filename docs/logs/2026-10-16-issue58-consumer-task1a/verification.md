# Author verification

All code checks use Go 1.26.4 through the unchanged local-only runner, offline
module resolution, credential environment stripping, and a rejecting proxy.
Commands run from `/bin/bash` with `login:false`. Metadata candidate HEAD is the
read-status prerequisite `46a10b3`; Task1a tests ran its explicitly hashed working
source, not immutable HEAD46a. The parent will rerun the committed candidate.

Literal runner prefix for root-module commands:

```text
python3 /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery <label>
```

Final source gates (append the following arguments to that prefix):

```text
issue58-task1a-final-normal go test -json -mod=readonly -p=1 ./internal/modules/memory ./internal/modules/diva-cognitive ./internal/contexthost ./internal/app -count=1 -timeout=35m -skip '^TestMemoryLoop'
issue58-task1a-final-race go test -json -race -mod=readonly -p=1 ./internal/modules/memory ./internal/modules/diva-cognitive ./internal/contexthost ./internal/app -run 'Issue58|TestToolkitMCPPublicationRefreshesAppToolHostAndRecovery|TestGatewaylessRun|TestHeadlessFormArmsMasks' -count=1 -timeout=15m
issue58-task1a-final-vet go vet -mod=readonly -p=1 ./internal/modules/memory ./internal/modules/diva-cognitive ./internal/contexthost ./internal/app ./internal/cognitivecontract
issue58-task1a-native-root-normal go test -json -mod=readonly -p=1 github.com/ProjectViVy/agent-vivy/bml -count=1 -timeout=5m
```

Normal: PASS380 test events +4 packages, 25.467s. Focused race: PASS81
test events +4 packages,70.878s, zero races; the opt-in SplitFixture is skipped. Existing opt-in SKIPs:
`TestCognitiveActionFixtureCapture`, `TestIssue58RecapSplitFixture`, and
`TestRealProviderSmoke`. MemoryLoop omission is the parent's documented phase
boundary, not a new test skip. Native BML under root MVS SQLite1.59:
PASS162+1 package, 1.727s. Vet: PASS, 2.085s. Normal's two attempted external proxy
requests were rejected; zero requests were forwarded. Race and native/vet use
zero external requests.

The native read-status prerequisite also ran the unchanged nested BML module
SQLite1.56 full normal/race:162+1 package each,1.576s/8.097s. The parent reran that
immutable source independently:162+1 each,1.579s/8.405s. These fixed46a results
are separate from working-source Task1a tests.

Raw logs preserve every RED/compile/setup/intermediate attempt. Consequential
REDs include the absent native status API, absent adapter, foreign Home reuse,
missing private binding, same-Home/different-core-lease constructor cleanup,
unchanged Toolkit Assembly reopen, actual App.Run leaving Home active, and an
incomplete native binding accepted by the adapter. One intermediate full-memory
failure was test setup (no DB yet) plus an obsolete foreign-root reuse assertion;
original output is retained. Two intermediate broad runs show the resulting
foreign-active failure chain, not independent product findings. An intermediate
Home-only early-listener defer was removed after an ordering review; its passing
logs are retained separately and not used as the final-source claim.

`git diff --check` and explicit-path gofmt checks are required before commit.
Actual Windows, browser, full repository CI, separate MemoryLoop/DIVA scripts and
Task1b source action qualification: NOT RUN in this author phase.

# Verification

Independent Linux-filesystem Debian WSL checkout:
`/home/mastwet/work/vivy-issue91-runtime/agent-vivy`, baseline
`01be84a64696ca921c376a4986606a01e1da4f91` plus the explicit D13 patch.
Own clean Laputa `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`, Go1.26.4,
Node24.21.0, pnpm10.33, GOFLAGS=-p=2. No dirty sibling checkout was reused.
No tenant Journal or production credential was read or written.

| Check | Result |
| --- | --- |
| Original packed minimal causal regression | RED: protected-tools absent from the seven-owner Recipe |
| Missing UI component regression with existing happy-dom | RED: component import absent |
| Host-created empty Session/workspace regression | RED before chooseWorkspace correction; GREEN after |
| Delayed admission and mixed result/failure regressions | 2 RED before review correction; included in final GREEN |
| SDK omission/Generation/Port/Host pressure checks | 71 other cases PASS, zero skip; actual default/minimal Pack and Inspect included |
| Packed minimal and minimal-plus-hello-fs causal checks | Both scenarios + parent PASS; actual documented bootstrap, code-face turn, exact persisted tool result in request2, one terminal and idle |
| SDK verify minimal-chat and hello-fs | Both PASS |
| Final minimal-chat CLI Pack/Inspect and selected UI staging | PASS; eleven Modules, correct exclusive root and en/zh catalog |
| just headless-compile | PASS, intentionally compile-only three packages |
| just wiring-ci | 16 Node + 111 Go PASS, zero skip |
| just kernel-ci | vet PASS; 1731 PASS, one existing optional rg skip; real minimal Pack/Inspect/boot1 PASS |
| UI component / PresentationHost / UI conformance | 45 PASS (7 new component + 32 Host + 6 conformance), zero skip |
| UI typecheck | PASS |
| Actual packed gateway + selected UI Vite3015 Playwright | 3 PASS, zero skip: causal protected list/read, cancel, provider failure |
| Desktop and 390x844 browser inspection | PASS; no horizontal overflow; Send remains in viewport; real tool output and final visible |
| Fresh independent review and correction re-review | PASS; no unresolved finding; see review.md |
| Source sync, formatting and git diff --check | PASS |

Final inspected mock-tested minimal-chat Generation identity:
`18d242b962af97a869e6b574fd153f39`. This is the build's opaque identity,
not a content digest. The Pack/Inspect check reads the sealed executable;
the browser suite uses that actual executable and an SDK-staged projection of
the same selected UI source, with no mocked RPC/store or second backend.

Commands, from the WSL checkout unless stated otherwise:

```sh
go test -json -tags=vivy_headless ./sdk/internal ./sdk/internal/assembly ./sdk/internal/conformance ./internal/toolhost -run 'TestMinimalCausalExchange|TestMinimalKernelArtifactBootAndInspect|TestMinimalRecipeOmitsOptionalCapabilities|TestCheckedInProviderConformanceMatchesExecutedSuites|TestGenerationFailureMatrixEvidence|TestGenerationRollbackRestoresCatalogAndLocaleIdentity|TestGenerationFailureMatrixExecutesEveryCase|TestMinimalArtifactPhysicallyOmitsOptionalModules|TestCompilePluginV1GraphFixtures|TestStartFailureRollsBackEveryConstructedOwner|TestMiddlewareTimeoutFailsClosed|TestMiddlewarePanicAndInvalidDecisionFailClosed' -count=1 -timeout 20m
go test -json -tags=vivy_headless ./sdk/internal -run '^TestMinimalCausalExchange$' -count=1 -timeout 5m
just headless-compile
just wiring-ci
just kernel-ci
go run ./sdk verify faces/minimal-chat
go run ./sdk verify plugins/hello-fs
go run ./sdk pack --recipe recipes/minimal-chat.vivy.yml --output .workspace/d13-minimal-chat-final
go run ./sdk inspect-artifact .workspace/d13-minimal-chat-final
go run ./sdk stage-ui --recipe recipes/minimal-chat.vivy.yml --output .workspace/d13-chat-ui-final
cd ui
pnpm exec vitest run src/plugins/minimal-chat.test.tsx src/plugins/presentation-host.test.tsx src/plugins/conformance.test.tsx
pnpm typecheck
# Absolute paths below point into this isolated checkout.
VIVY_MINIMAL_CHAT_ARTIFACT=/home/mastwet/work/vivy-issue91-runtime/agent-vivy/.workspace/d13-minimal-chat-final VIVY_UI_ASSEMBLY_ENTRY=/home/mastwet/work/vivy-issue91-runtime/agent-vivy/.workspace/d13-chat-ui-final/assembly.ts pnpm exec playwright test --config playwright.minimal.config.ts
# Canonical Windows checkout:
git diff --check
```

The initial combined Go run had 72 passed cases and failed only the hello-fs
scenario/parent: the normal enabled-tool ceiling correctly refused hello_stat.
The example config and test now explicitly enable it. The two corrected
scenarios and parent subsequently passed, including the final code-face
submission; already passed unrelated pressure cases were not repeated.
The final UI-only changes do not change kernel owners or compiler wiring.

The mock records raw model requests only in test memory. Three streaming Run
requests prove list result then read result reaches the next request. The
existing separate nonstreaming 40-token ChainTitler request gets a deterministic
title and does not consume the Run script. The nonce is absent from the prompt
and comes from the real workspace read. Each browser case reads committed Run
logs and background/list through the existing browser API client, verifies one
matching terminal and zero active background Runs. The hanging model is closed
by test cleanup; no key is needed. Expected fixture error-boundary output in
the pre-existing PresentationHost negative case is not a product failure.

Environment failures were not counted as passes: initial jsdom selection was
corrected to the repository's existing happy-dom; matching Chromium was missing;
then WSL lacked shared libraries and fonts. Chromium was downloaded to its user
cache. Debian packages were downloaded/extracted into task-local scratch with
LD_LIBRARY_PATH and FONTCONFIG_FILE; no system package or global Go setting was
changed. After those repairs all three browser scenarios actually executed.

The kernel skip is the pre-existing TestBackendGrepRipgrepHonorsGitignore because
rg is absent in WSL. It is not counted as passed. D05's full kernel race gate
already passed at its accepted predecessor; D13 changes no runtime concurrency
owner and does not claim another race run. No database implementation/schema
changed, so no new PostgreSQL or conversion claim is made. No Windows full CI,
native packaged Windows smoke, full-product just ci or real-token check ran;
these remain separate D14 acceptance lanes under the owner's explicit scope.

Residue: no new hash/digest mechanism is introduced by this Story. Existing
continuity download-check helpers are unchanged test utilities. D09 owns file
preconditions, D11 workflow revisions, and D12 migration metadata; retained core
loop behavior belongs to #93 extraction. No old-schema/approval fallback added.

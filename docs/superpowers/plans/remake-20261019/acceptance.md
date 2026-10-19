# Focused verification and final acceptance

## Stage rules

Planning checks are scope/coverage/link/DAG checks. No product tests run for this artifact.

For implementation, use the current target's actual instructions. Do not copy old-main commands requiring absent assets/modules. Prepare dependencies and UI embed only when the selected target needs them; identify the command that performs that preparation. Reject zero-discovered-test selections. Run a meaningful failing assertion before a fix, then the focused passing selection and affected contract checks. Do not repeat unchanged expensive gates after evidence is sufficient.

The 2026-10-19 #91 owner decision allows L0/L1/L2 iteration and a final base `just ci`; external/real-path evidence stays pending at its designated stage. That decision is scoped to the base cut. Remake retains the accepted target's mandatory gates, and a request for minimum verification is not a silent CI/data/permission waiver.

| Change risk | Minimum useful iteration evidence | Before capability acceptance |
|---|---|---|
| Docs/ledger only | Package verifier, link/coverage/DAG and diff check if applied to repo | Correct facts and explicit unknowns |
| Local Go behavior | fmt, focused affected vet/test; one actual failure path | Current target required candidate gate |
| Shared loop/RPC/approval | Meaningful causal path + cancellation/refusal and affected concurrency/recovery cases | One production model/tool/result/call-ID/next-request/terminal/idle path; required candidate CI |
| Optional package | Selected behavior and explicit omitted capability path | Pack/Inspect source-first artifact evidence; no unselected executable implementation where required |
| UI behavior | Typecheck/unit/build on touched source and real backend action flow | Real split browser flow with isolated home/ports; capability truth matches backend |
| Persistence/migration | Disk close/reopen, conflict/replay and applicable crash cuts | Fresh + supported source upgrade + failure rollback + paired SQLite/Postgres parity; BML format gates where affected |
| External provider | Outbound request assertions and deterministic stream/failure fixtures | Required race/real-provider evidence at the agreed stage; paid calls require actual authorization |

## Common causal regression

The accepted production Service/RPC/provider/tool path reads an isolated nonce with a real tool. Capture model request two and assert the exact tool result and call ID; then final text, one terminal and idle cleanup. Cover a refused or model-recoverable tool result and cancel; if the batch touches approval/restart/ownership also exercise those cuts. Scripted model fixtures prove plumbing; they are not real-model quality evidence.

## Data and concurrent behavior

Use disposable copies, one lane home and controlled failure injection. Never production Journal/data, automatic resets, released migration edits, or blind replay of unknown effects. Test same-length stale content/revision conflicts where the feature owns them, not just different-size edits. A feature omitted from a Recipe must fail truthfully as unavailable rather than exposing a fake success UI.

## Cost and performance

Reuse the accepted minimal-loop benchmark fixture and repeat under comparable machine/model/input/tool settings. Record wall latency, call counts, optional maintenance activity and allocations where the repository already measures them. Identify environment noise and use no invented percentage threshold. A measured regression gets a disposition before integration; do not claim an improvement from static simplification alone. Parallelism requires a deterministic overlap probe in addition to timing.

## Exact candidate and remaining evidence

Run applicable mandatory CI once on the final candidate, with new runs justified by a changed candidate/failure/risk. Root `go test ./...` does not cover nested BML/plugin/Face modules; use the actual current module gates. Main donor has `just ci`, `just plugin-ci`, `just bml-ci`, `just test-postgres` and Toolkit real-browser entry; availability on B must be inspected rather than assumed. PostgreSQL skip is pending, not parity. Cross-compile is not native Windows runtime evidence. Old receipt + new branch is not a new pass.

No feature is product-accepted while a required gate remains pending; engineering handoff may explicitly carry that pending gate to the authorized PR/upstream acceptance stage. Do not close original issues based on this plan, a merge, a source scan or a green build alone.

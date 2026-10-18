# PR #79 review fixes (issue #58 integration)

Follow-up to the review of ProjectViVy/agent-vivy#79 and ProjectViVy/laputa#10.
Branch `claude/gallant-pasteur-g3f9qq` is PR #79's head plus the commits below.

## What changed

| Area | Change |
|---|---|
| Invalid UTF-8 tool arguments | The unavailable-evidence marker, the dispatch rejection and the projector rejection now apply only to runs that pin subject evidence. An ordinary or coding session that streams non-UTF-8 arguments stays readable. |
| Recoverable tool errors | In a Recap run an argument error, a missing file or a refusal stays a model-visible result. Only a hard operation failure remains a run error; the evidence still records the original error. |
| Stuck terminal runs | A run whose tool operation stayed claimed, or whose arguments are not valid UTF-8, seals as an `excluded` interaction (migration 046). Unrelated seals, context capture and Recap context no longer fail because of it. The condition is re-derived inside the sealing transaction. Contract paragraph added to `VIVY-UNIFIED-CONTEXT-MEMORY.md`. |
| Observers | Memory and cognitive capture skip runs with permanently unknown input provenance and advance the cursor, instead of retrying them forever. |
| Recap in the default build | The control plane advertises `recap` only when a main-conversation resolver and a Recap owner exist. The Memory page keeps its previous view otherwise, and Recap reason codes are translated. |
| Composer thinking control | The label and the sent level agree: a level the model does not offer is dropped; a restored queued level is shown. |
| Quick approval card | Names the session when the request is not from the active one. |
| Small | `memory_search` clamps to 100 instead of rejecting a schema-valid limit; `tool.requested.json` describes v1 and v2; SQLite `DeleteSession` removes `message_copy_sources`; `run_subject_sources(session_id)` index; BML comment corrected. |

Laputa (separate repository, delivered as a patch): a canceled context during first-use Recap recovery no longer marks the owner commit-unknown; `golang.org/x/sys` declared direct.

## Not done

- Production resolver wiring: its own feature branch (`MEM-58-RESOLVER`).
- Re-pinning Laputa: needs Laputa #10 merged to `main` with a merge commit first (`MEM-58-LAPUTA-REPIN`).
- Laputa chain re-validation cost: measured, not changed, because a cache removes a per-read corruption check an existing test pins (`LAPUTA-RECAP-CHAIN-REREAD`).
- Journal replay per tool call: unmeasured (`RUNTIME-TOOL-JOURNAL-REPLAY`).
- `RegisterAdmissionTx` conversation-id clash: not changed. A second subject cannot reach it because admission requires the one registered subject identity.
- PostgreSQL runs were not available; 046 is covered by the SQLite apply test and a SQLite/PostgreSQL text-parity assertion.

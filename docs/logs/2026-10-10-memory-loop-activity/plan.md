# Native terminal activity continuity

Baseline MEM-S05-01 was RED: two real completed primary turns preserve raw canonical input but leave native Markdown Pulse/Recap empty. This is local product wiring work. The earlier untracked App probe and v0.7 raw proof remain intact.

The native ACTMEM owner will append a bounded Pulse/Recap pair atomically from a trusted primary terminal capture's redacted user source. Metadata comes from the original durable ingestion identity/sequence and trusted profile/session/workspace binding. User content remains quoted/escaped data; assistant/tool/system text never becomes the recap source. Do not create authority through raw ingest rows or use ACTMEM as automatic recall context.

Use the existing ingestion SQLite ledger only for typed source intent/operation receipts, not another ACTMEM authority. New host-only capture metadata opts fresh events into projection; previously accepted rows stay unchanged. Track pending, applying, applied and unknown projection status. The worker uses one native head snapshot/CAS for the atomic pair and records the real entry IDs/revision after success. No blind reappend after a possibly committed Markdown mutation: reconcile exact entries in head/capsules; a matching unchanged pre-write head proves no effect, otherwise retain an explicit unknown fence. Later source watermarks cannot cross unresolved activity projection.

No ACTMEM v2 grammar/header change, dependency repin, new Runtime/model, global owner, persona write or generic model maintenance privilege. Keep the current global head revision and verify concurrent captured activity versus Work reconciliation; do not weaken existing revision checks. Store and fixture tests distinguish native kernel proof from complete App acceptance.

First implement native snapshot/atomic-pair/receipt lookup with real file recovery tests. Then connect the native ingest worker and Runtime's admitted redacted user source. Re-run actual two-turn Pulse/Recap continuity, terminal variants, restart/redelivery identity, no automatic foreground injection and existing real composition. Archive requires a real session-delete barrier that drains terminal capture before deleting messages, then folds the original session; a direct FoldSession call is not acceptance. Same-operation recovery and crash cuts remain their own required proof, not a claim from happy-path continuity.

Failure/limits: unclassified/read-only/malformed heads refuse projection; unknown effects retain their source identity and stop watermark advancement. Operator edits or eviction can make an unreceipted effect unprovable; preserve that ambiguity. Source worker ordering and bounded retry are explicit requirements. Keep formal S05/S06/S10/S11 open until complete same-candidate gates and matrices pass. Goal continues; no external-machine deferral of this code work.

## Execution decisions

Use pinned Eino v0.9.13 through the existing Service/ADK inference path. This increment adds no inference implementation: the existing admitted MessageStore rows and Journal terminal events supply trusted capture source; the existing ObserverHost supplies durable delivery/cursors. Eino callbacks alone cannot replace host source admission, native Markdown receipts or the Session deletion boundary, so keep those adaptations in their current owners.

A real native archive reader rejected oversized capsules written by FoldSession. The native owner preserves its existing schema/cap and complete source metadata through compact equivalent YAML, preflights unrepresentable single sources, and bounds captured bodies to the remaining capacity. Raw admitted source is retained whole.

Session deletion seals admission, permits only terminal persistence while closing, waits for actual primary terminal status, drains the same ObserverHost, then waits/folds native source projections before deleting messages. Failure leaves the Session sealed and its source intact. Serialize worker and explicit Observer drain with a context-aware gate; do not call the sink directly or fabricate terminal IDs.

Ruling: the bound durable source alone decides the admissible watermark; accepted-source notifications are hints. Ruling: unchanged Work permits the expected capture revision increment, with every original Work metadata field preserved; normalize equivalent empty source slices in the assertion. Restart rehydrates the actual Session before its authorized actions. These are test/contract corrections, not relaxed production Work CAS.

The final activity observer waits for the owned native source prefix, not merely the raw canonical write, because those are separate asynchronous boundaries. Final combined diagnostic regression: 15 pass/zero skip/exit0. Formal Story gates and local remaining matrices stay open.

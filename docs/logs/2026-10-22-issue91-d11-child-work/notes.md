# Finite conversion mapping and historical residue

The separate offline converter remains the only supported old-source conversion venue. These are the exact owner mappings for baseline 035; no unlisted source formats or post-baseline heads are accepted by this lane.

| Baseline owner | Target disposition |
| --- | --- |
| `child_sessions.request_digest` | Delete column. Add nullable private `request_input TEXT`; every baseline row remains NULL. A historical Message or later activation is not proof of the original normalized admission input. |
| `child_sessions.authority_ceiling_digest` | Delete column. Retain existing canonical `authority_ceiling_json` as the sole actual authority value. Validate its typed policy/sandbox/approval/tool shape; no digest verification or reconstruction. |
| `child_sessions.activation_request_digest` | Delete column. Add nullable private `activation_request_input TEXT`; every baseline row remains NULL. |
| `child_session_activations.request_digest` | Delete column. Add nullable private `request_input TEXT`; every baseline activation remains NULL. Preserve activation Run, authorizer, operation key, tools and timestamps. |
| `session_work_events.request_hash` | Delete column. Add nullable private `request_input TEXT`; every baseline receipt remains NULL. Preserve request ID, sequence, lifecycle values and raw payload bytes. |
| Historical `session_work_events.payload.Mutation.RequestHash` | Preserve immutable raw payload bytes as inert historical data. The new domain field has no such property and decoding does not read, compute, validate or reuse it. This is a historical residue exemption, not runtime authority. |
| Child mailbox messages/receipts and cursor columns | Preserve every body, status, sender/recipient, sequence, cursor, receipt and consumed-by lineage without rewriting. |
| Work Goal/Plan review state | Preserve lifecycle event history and replay it as before. NULL receipt input refuses idempotent mutation reuse. Existing old-run/checkpoint conversion belongs to D09 and remains authoritative for execution recovery. |

Unknown child input does not close the child, discard messages or invent an execution state. It fences admission reuse/continuation/direct start/recovery and requires explicit review or a new correctly admitted child. Unknown Work input does not erase pending review or historical state; it refuses reusing that old request identity. Source rows never infer exact original input from old digest strings, effective state, task history or equal lengths.

Paired migration 046 has identical logical column ownership in SQLite and PostgreSQL. Released migrations 001..038 and the other owners' assigned migrations were not edited. This log specifies the fragment for the separately delivered offline artifact; it does not claim a baseline conversion artifact or end-to-end conversion fixture was shipped by this lane.

Notes/notebook prose called a digest is a textual summary, not cryptographic authority. Such prose is not deleted by this closure.

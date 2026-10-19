# Issue #91 detailed architecture

The owner requested detailed architecture after confirming complete custom-hash
retirement and species-selected retention of nudge, warming and steering. Extended
the existing shared deletion design with §11; no competing spec was created.

The design fixes ownership/import direction, three closed T1-only factory Ports,
pure SDK policy signatures, selected Eino adapters, actual tool-to-model flow,
retry/resume/cancel and failure behavior. It fixes finite SQL/BML/approval/
checkpoint/bundle/Studio ledger source and target formats, direct-input equality,
NULL legacy binding, copied-target migration/cutover and required consent renewal.
Updated all 15 Story references and affected contracts/readiness in the index;
the 15-node, 26-edge, six-wave DAG remains unchanged.

Read-only inspection of exact INOFY/Laputa/Studio pins exposed real required
upstream changes. D10/D11 remain blocked on coordinated hash-free upstream APIs;
D06/D07 require affected Studio consumer closure. No upstream source/pin was
modified. Two read-only domain reviews found concrete interface/legacy-state
issues; those corrections are incorporated in the same design.

This delivery changes documentation only. Runtime code, generated files, schema,
installed data and product dependencies are unchanged. It neither starts product
implementation nor closes issue #91.

# D12 BML format and direct memory consumer

Status: implementation in progress; D12 acceptance remains open.

This isolated data lane starts from stable `e2c1518e`. It advances D12 before
D09, D10 and D11. Root D03 changes and migration 036 remain uncommitted inputs;
this lane does not copy them or modify overlapping migration runner files.

Implemented candidate: hash-free BML schema2/artifact2.0.0, opaque new IDs,
persisted workspace ownership, read-only startup guards, exact import admission
and restart allocations, obsolete-journal omission and bounded owned rollback.
The MEMRULES direct consumer uses exact text/existence and preserves conflict
refresh instead of depending on the removed BML digest helper.

Ruling: the sole external BML digest-helper consumer (MEMRULES CAS) moves into
D12's producer/consumer closure. It consumes no D08 snapshot API; this prevents
a compile dependency cycle without accepting or starting D10's broader work.
No duplicate hash implementation is moved into the memory Module.

The finite BML converter is delivered separately as `f7875b21a65b5a45f72ce83e084f4da300b45487` on `codex/issue91-d12-rules`; it must not
be merged into the final runtime. Application metadata/checksum removal waits
for committed D03 integration. D14 assembles the complete upgrade artifact;
no final application target migration number is reserved here.

No production data, released SQL, runtime/provider orchestration, or active
D03 root work is changed. D09 waits for accepted D03; D10 waits for accepted
D08/D12 and compatible Laputa; D11 waits for accepted D03/D08 and compatible
INOFY. Current pinned Laputa/INOFY still require digest authority.

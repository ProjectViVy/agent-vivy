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

The finite BML converter is delivered separately as `47e5a54956b4411a51ec7088b61c2811d0bceeb2` on `codex/issue91-d12-rules`; it must not
be merged into the final runtime. Application metadata/checksum removal waits
for committed D03 integration. D14 assembles the complete upgrade artifact;
no final application target migration number is reserved here.

No production data, released SQL, runtime/provider orchestration, or active
D03 root work is changed. D09 waits for accepted D03; D10 waits for accepted
D08/D12 and compatible Laputa; D11 waits for accepted D03/D08 and compatible
INOFY. Current pinned Laputa/INOFY still require digest authority.

D03 has since committed and been integrated; metadata work is tracked in the
[metadata continuation](../2026-10-21-issue91-d12-metadata/summary.md).
The corrected finite BML converter verifies reconstructable RecordsDigest and
preserves released dangling/cyclic link graphs. Its bridge to the actual target
passes both cases after the corrections. InputDigest remains unverifiable.

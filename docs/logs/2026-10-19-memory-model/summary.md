# Memory model verification

Added an opt-in standard-library Python reference model, fixed-seed traces, strict
JSON worker protocol, replay receipts and native Go overlay bridges/regressions.
The bridges exercise real BML bound SQLite operations and pinned ACTMEM Recap
storage, including clean reopen, fresh processes, abrupt exit and recovery faults.

Production behavior, Laputa pin, schemas, runtime, ownership guards and aggregate
CI selection are unchanged. Python never writes storage or authority files. This
is a storage-verification slice, not autonomous-evolution or product acceptance.
No release or installation is involved.

The current pinned Recap defect remains a real failing result. It is not skipped
or converted to an expected pass. See verification.md for controls and boundaries;
usage is in scripts/testdata/memory-model/README.md.

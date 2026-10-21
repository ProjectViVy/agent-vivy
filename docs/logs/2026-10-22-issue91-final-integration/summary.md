# Issue 91 final integration

Phase 1 integrated and verified. The owner authorized D09/D12 integration, then D10/D11 downstream
adaptation to the merged upstream contracts, then D14 final acceptance.

The single write lane starts at stable cd0d4d3f and consumes data-feature
50d63a32. Stable SQL 001–038 is preserved. D09's unpublished file/receipt/
presentation changes use paired039–041. D12 remains the owner of checksum-free
metadata and BML2. No donor scratch database is promoted to a supported format.

The source-format preflight now recognizes accepted stable old-metadata037/038
by actual catalog names and schema, refuses writes and requires the finite
offline upgrade. It adds no legacy checksum reader or runtime converter.

The actual split-browser path also exposed and corrected cross-workspace read
marker ownership. Markers now bind the resolved private root; user history
keeps relative paths. Actual absent reads restore editing authority without
trusting old rootless markers. Nested missing directories remain confined,
and errors do not disclose private keys. Independent review cleared the fixes.

D09/D12 scoped acceptance passes the required Linux wiring/kernel/Face/Web,
real PostgreSQL, BML, affected race and actual3015 browser paths. The existing
optional rg-dependent skip and final release limits are recorded separately.

D10/D11 adaptation, the complete finite upgrade artifact and all D14 final
verdicts remain pending. No production Journal or tenant home is accessed.
No push or product release is claimed by this record.

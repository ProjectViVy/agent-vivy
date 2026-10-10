# Source-bound conformance refresh

The memory repair changes the canonical internal source tree. Updated exactly the five internal `sourceSha256` entries in the checked-in conformance bundle to the stable source-hash helper result, without changing provider IDs, checks, evidence IDs or pass flags. Diagnostic test logging was removed.

No release; candidate rebuild and required CI follow the same frozen source set.

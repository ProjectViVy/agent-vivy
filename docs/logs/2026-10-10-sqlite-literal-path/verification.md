# Verification

`TestOpenPreservesLiteralDatabasePathIdentity`: RED, fragment/query names read the first profile's snapshot and percent names created a different literal file. GREEN after URI encoding.

Full SQLite package: 158 named tests/subtests pass, zero fail, zero skip, exit 0. The runtime settlement tests that previously inherited the malformed temporary path now run against their requested isolated database. Subsequent native Windows runtime verification and final source-bound/full CI gates remain pending; Linux results do not attest Windows execution.

Raw records: task logs `chain-sqlite-path-identity-red.jsonl` and `chain-sqlite-path-identity-regression.jsonl`, copied to the paired DIVA recovery checkpoint before handoff.

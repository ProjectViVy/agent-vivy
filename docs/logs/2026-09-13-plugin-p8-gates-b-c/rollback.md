# PLG-P8 Gate C rollback evidence

Date: 2026-09-13

The rollback test packs two real executable artifacts:

1. a prior default Generation;
2. the SCX candidate Generation
   `40c2e3c2302a23b277bab5db159d2bbd59d8af60477ce6f568ce81c5d5d3e5a5`.

It launches the candidate in an isolated evaluation root and requires a mixed
health verdict, proving that the generated SCX Assembly boots. It then creates
real Studio ledger records and release records for both artifacts, installs the
prior release, installs the candidate, and verifies the embedded candidate
Generation identity from the installed binary.

`Studio.Rollback` restores the prior release and the test re-extracts the
installed binary's embedded manifest to prove the prior Generation identity.
A sentinel written to the tenant Journal path before either install remains
byte-identical after rollback. Release state and durable tenant truth are
therefore separated in the exercised path.

Removal is independently covered by packing `recipes/minimal.vivy.yml` and
verifying that its sealed Manifest, dependency graph, generated Assembly, and
binary omit the SCX Module. Rollback does not dynamically unload code; it
atomically restores the previously sealed whole Generation.

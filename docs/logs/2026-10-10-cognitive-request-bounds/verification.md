# Verification

`TestCognitiveInferencePreservesCompleteBoundedRequest`: RED, complete source JSON was lost; GREEN after removing the unrelated authored-task slice. The test checks the entire Unicode source JSON, output schema suffix and existing native child bound.

Focused cognitive/INOFY/workflow/one-shot tests passed 75 tests/subtests with zero skip before the paired packet repair. That repair reduced intermediate bytes enough that the old aggregate-pressure fixture no longer exceeded its 8 KiB policy; its regression correctly reported this mismatch. Increase only that fixture body from 1.2 KiB to 2.2 KiB to continue exercising the old persisted aggregate ceiling. Both old-limit rejection and new-limit execution assertions remain unchanged.

`TestMemoryLoopAutomaticReflectionLargeSource`: initial real App RED at reflect with an incomplete window; the isolated canonical source existed, but no applied memory proof. After the paired Laputa packet repair, GREEN verifies the whole supplied user body in actual recorded model requests and canonical effect, actual effect receipt, processed source watermark, and exactly one raw source plus one memory effect.

Final directly affected regression counts and raw logs are recorded in the paired DIVA checkpoint. The diagnostic overlay is not a new-source sealed artifact. Final complete CI/conformance reproduction/pack/Inspect are pending for this phase; older checkpoint results do not attest these commits.

Recorded post-fix regression: runtime 75 pass, selected actual composition 11 pass, Laputa 64 pass and Garden 480 pass. All four exit 0 with zero fail and zero skip. The failed large-source run and the pressure-fixture mismatch remain preserved as separate diagnostic records.

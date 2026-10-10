# Acceptance

Run `just diva-recall-test` after the repository's embedded UI assets have been built (the normal CI recipes build these first). All six required test names must report PASS under the SDK-generated DIVA assembly. Skips and absent tests fail the command.

Ordinary default-generation tests must retain the cognitive owner without selecting native recall. `TestDefaultGenerationOmitsNativeRecall` passes; the six recall-only tests skip that generation. This preserves opt-in composition while making their actual DIVA execution mandatory in both CI recipes.

The native memory injection, restart recall, negative controls, authority boundary, deadline degradation and correction/deletion assertions remain unchanged.

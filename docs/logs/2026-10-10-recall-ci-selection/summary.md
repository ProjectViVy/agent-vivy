# Recall CI selection

Six native recall tests previously gated only on the cognitive factory. The default generation includes that owner but intentionally omits the optional native recall source, so ordinary CI ran those tests with no native source and failed with empty query traces. The same composition and guards were present before the M1 metadata extraction.

The six tests now require both the owner and the generated `vivy.memory.mentle` source. A default-generation regression checks the distinction, including a source-free assembly. Other memory-loop tests retain their existing selection.

`just ci` and `just backend-ci` require `diva-recall-test`. Its PowerShell script generates `recipes/diva.vivy.yml` through the existing SDK catalog/compiler/assembly generator, applies a temporary Go overlay, and executes all six tests with `vivy_diva_integration`. Any skipped or missing selected test fails the gate. Unique temporary files are removed in `finally`; JSON is BOM-free on Windows PowerShell and PowerShell Core.

The generator accepts an optional `--recipe`, preserving its default recipe and existing form identity behavior. No recipe, committed generated assembly, runtime behavior, native source, dependency pin, conformance pass flag or source hash was changed. Scripted model responses establish integration behavior, not live-model recall quality. Release and artifact attestation remain the integration lane's responsibility.

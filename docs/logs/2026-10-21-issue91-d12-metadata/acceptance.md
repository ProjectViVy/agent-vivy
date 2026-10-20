# Human acceptance

On disposable stores, fresh startup produces exactly version/name/applied_at
metadata and complete application schema. Reopen is a no-op. Inject failure
mid-bootstrap and confirm no partial schema survives.

Old checksum/legacy metadata requires the finite upgrade tool before startup.
Unknown or incomplete target stores refuse without modifying their bytes.
Convert copies through the separate frozen fragments, then open the target
runtime and verify history, NULL source bindings and unknown-effect fences.

D12 acceptance is open pending exact integrated gates, source-format conversion
fixtures and split UI at3015. D14 complete conversion/release acceptance is
separate, and later Story prerequisite gates are not waived.

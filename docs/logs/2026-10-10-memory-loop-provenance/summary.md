# Actual memory provenance observation and process retention

Extend the test-only real App observer to read source_json and metadata_json from the same canonical row/version matched to public effect receipts. Record full evolution SourceRefs, primary native locator/revision and the public bound source/scope. Automatic reflection assertions join those fields to the actual ingested source sequence; no source identity is filled from the expected random answer.

An owned-process Restart test now retains the ordinary memory body, record/revision, operation identity, full references and native locator/revision across distinct PIDs, with zero new model requests in the new process. The paired Garden/Mentle product repair preserves these fields during native creation/update; Vivy changes here are test infrastructure only.

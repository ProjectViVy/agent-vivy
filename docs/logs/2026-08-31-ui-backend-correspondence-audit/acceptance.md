# Acceptance

1. Open the report and, by P1/P2, file, and line number, reproduce the evidence for each case of “backend exists but the UI does not correspond.”
2. The report's “verified correspondence” and “explicitly excluded” sections do not misreport demo features that are absent from the channel or backend as defects in this delivery.
3. Run `just ci` to reproduce the gate result; after starting the split pair, both health/home URLs return 200.
4. The report explicitly states that the browser runtime is unavailable and does not treat HTTP 200 as a complete visual smoke test.

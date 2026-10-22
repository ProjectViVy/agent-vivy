# Verification

At executable2364d8f6, the original actual packed masks suite passed nine cases
and failed two after session-creation callbacks changed the page mid-test.
The exact helper correction was applied temporarily in the independent Linux
checkout against the same inspected Generation, with all existing assertions
retained:11 browser cases pass (59.2s), including reload, cross-tab revision
conflict, delete-in-use refusal, active-Run switching and narrow layouts.
The temporary copy was restored byte-for-byte after testing. The helper is
now committed so future runs use the same deterministic preparation.

Independent source review found that message readiness can precede the final
creation callback. The helper also waits for the existing sessionBusyId to
clear, while retaining initial composer visibility for cold UI startup. The
two affected real browser cases pass again (24.5s) with this final correction.
No product code, retry, assertion or deadline changed. Standard UI typecheck
also passes; the browser runtime execution is the evidence for this e2e helper.

# Acceptance — P8 review fixes

How a human can tell the fixes worked, without reading the diff:

1. **Retry backoff**: with an SCX Generation that selects a Run Observer
   which keeps failing (for example a receiver at capacity returning
   `DeliveryFailed`), the process no longer wakes every second forever:
   retries space out to at most once a minute. Delivery still resumes on
   its own the moment the receiver recovers — the existing reconnect
   behavior is unchanged.
2. **Expired required context**: a context source that marks a candidate
   `required` (or `reserved`) and lets it expire now fails the model call
   with `contexthost: required context expired` instead of quietly running
   without the context it declared mandatory. Ordinary (competitive)
   expired candidates are still just omitted.
3. **Resume performance**: a long run with many tool-call approvals no
   longer re-reads its whole Journal on every resume just to recover the
   Context View; resumes after the first one hit a per-run cache. If the
   Journal scan ever fails mid-read, a warning line names the run and the
   resume proceeds without a View.
4. **Generated observer policy**: `vivy-sdk pack` output (and Inspect) no
   longer lists a `result` payload field in the sealed Run Observer policy;
   the visible allowlist matches what terminal payloads actually contain.
5. `just ci` green is the overall gate for the tree.

Round 2 (foundation hardening):

6. **Firewall**: a module source importing `plugin`, `syscall`,
   `golang.org/x/sys`, or `unsafe` is now rejected by `vivy-sdk` source
   verification with `forbidden source import ...`, exactly like the
   existing runtime/Eino rejections.
7. **Unverified T2**: a T2 module record that reaches the compiler without
   a hashed, pinned source root fails the build with "lacks a verified
   source root" instead of compiling silently.
8. **Stale recipes**: a Recipe whose `grantApprovals` names a module that is
   not in `modules` fails with an explicit diagnostic instead of quietly
   ignoring the approval.
9. The Module Standard now says out loud that the source firewall is
   advisory defense-in-depth, so Inspect and docs can no longer oversell it
   as a containment boundary.

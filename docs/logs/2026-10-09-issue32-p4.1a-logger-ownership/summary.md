# P4.1a owned host logger

## Outcome

Implemented H4 on `feat/issue32-remediation`. Code commit: `a0c2bbff`
(`fix(host): inject the owned logger before composition`).

`app.WithLogger` lets a host supply the logger it owns before App composition
starts. The sealed VIVY host installs that logger before opening the embedded
runtime, so composition and component logs use the selected profile. The host
remembers the prior default and restores it only while the process default is
still its own logger. On close, it restores the default, closes its sink, and
only then releases the process owner slot. Failed startup follows the same
ownership cleanup and preserves a newer logger installed by another owner.

The daily file writer becomes permanently closed after its first Close. A
retained logger can no longer reopen a previous profile's log file or write
into a later profile.

## Evidence

- App composition log capture distinguishes an injected logger from a prior
  process default.
- Sealed-host profile A -> close -> profile B tests inspect both profile log
  directories and write through the retained A logger.
- Injected post-logging startup failures verify sink closure, owner-slot reuse,
  default restoration, and preservation of a newer process logger.
- Full `internal/app` tests and race-enabled logging/host suites passed; see
  [verification](verification.md).

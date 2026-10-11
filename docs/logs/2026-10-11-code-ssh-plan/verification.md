# Planning verification

Baseline: `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`; date: 2026-10-11.
Verification scope is this documentation package, not worker/plugin functionality.

## Focused checks

A Python document verifier executed successfully: 17 changed documentation files,
nine complete Story files, four Epics, 63 resolved local links, balanced code
fences, complete R1-R9/A01-A10 coverage and eight existing baseline test names.
It parsed the authoritative index, rejected unknown/self/cyclic/redundant edges,
and derived the seven waves recorded there. SSH-04 through SSH-08 remain Blocked.
Changed-path scope contains only this package/logs and the one TODO pointer.

Manual review checked worker/plugin interface consistency, exact Port consumers,
source boundaries, shared-file ownership and explicit proposed path labels.
Corrections include foreground polling to preserve lease/cancel availability,
secure cwd-handle launch, same-uid receipt trust limits, literal host-key storage
acceptance and generic selected-Recipe UI staging. No gap is marked proved by
these document changes. `git diff --cached --check` passed for all 17 explicitly
staged paths; a fresh link/header/fence check also passed before publication.

## Research and contract inspection

Read root/UI instructions, repository plugin skill, four normative standards,
Supermanagement architecture/coordination/Story references and Superpowers
planning guidance. Inspected actual public Ports, generator constructors, source
verifier, Spawn facade, runtime admission/middleware/replay, local instructions,
preview/delivery and UI staging/action/header APIs. Existing test names referenced
by SSH-00 were found in baseline source; they were not run in this planning turn.

Issue #15 was re-read through the authenticated GitHub connector; it remains
open with zero comments and records planned capability rather than implementation.
The issue's current body and implementation boundary match the plan's optional
remote scope. No issue/comment mutation occurred. Remote code-ssh ref was read
back at the exact baseline before documentation publication.

Pinned Eino checks are recorded in SSH-D1 §8. OpenSSH primary manuals were
checked for fixed configuration, host-key handling and the limits of SSH-session
closure as cleanup evidence. These inspections support design decisions only.

## Checks intentionally not run

No product source, dependency manifest, UI, Recipe or public/internal contract
changed. No dependency installation, runtime start, SDK pack, full `just ci`,
worker tests, live SSH or GUI acceptance was run. Those commands are executable
implementation/release gates in the Story plans; their outcomes are unverified.
Documentation checks must never be substituted for A01-A10 product evidence.

## Publication

Publish only these reviewed documentation paths to code-ssh using a non-force
ref update leased against the baseline. Verify remote parent, human author,
changed paths and final ref after publication; report that commit to the user.

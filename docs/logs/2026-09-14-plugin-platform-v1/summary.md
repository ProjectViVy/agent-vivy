# Plugin Platform v1 release summary

Date: 2026-09-14

Status: local release gates complete; GitHub Actions and the Windows browser
smoke remain required before PLG-1 closes.

PLG-P9 makes release support an evidence-derived property of the sealed
Generation. A public Port is reported as `SUPPORTED` only when its exact
Provider, Port, source digest, required artifacts, and complete executable
conformance record agree. Descriptor prose and hand-written pass labels cannot
promote support.

The release slice includes:

- a reusable 15-check Provider harness and 23 exact Provider/Port/source
  producer cases;
- compiler-owned catalog completeness and source-bound Inspect evidence;
- an executable 25-case failure matrix with exact redacted diagnostics and no
  artifact on every rejected build;
- default parity, minimal physical removal, selected full-UI, and SCX release
  proof;
- whole-Generation Studio rollback that restores the prior embedded Manifest,
  catalogs, locales, and executable without mutating tenant Journal truth;
- corrected v1 developer workflow and CI coverage for the embedded UI build;
- a repository-skill load gate discovered by the credential-free VIVY CODE
  smoke.

Independent code review reported READY on the final local tree, published to
GitHub as `5304831`, with no Critical, Important, or Minor findings. PLG-1
deliberately remains on the open board until the PR's complete GitHub gate,
including Playwright, passes.

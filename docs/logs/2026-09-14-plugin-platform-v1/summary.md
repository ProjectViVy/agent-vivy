# Plugin Platform v1 release summary

Date: 2026-09-14

Status: PLG-P9 complete; the published code head passed all GitHub release
gates in [run #147](https://github.com/ProjectViVy/agent-vivy/actions/runs/34825393484).
PR [#27](https://github.com/ProjectViVy/agent-vivy/pull/27) remains open for
human merge; its `Closes #14` body will close the tracker issue on merge.

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

Independent code review reported READY with no Critical, Important, or Minor
findings on the final tree, published through `2e715cf`. PLG-1 is archived
from the open board after the complete GitHub gate, including Playwright,
passed.

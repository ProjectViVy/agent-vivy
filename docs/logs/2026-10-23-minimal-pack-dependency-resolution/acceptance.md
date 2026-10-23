# Acceptance

Run `just kernel-ci` from an independent Linux-filesystem checkout. The required `TestMinimalKernelArtifactBootAndInspect` must pack, inspect, and launch the production minimal Recipe without fetching source archives for unrelated UI modules. The build must continue to verify every downloaded module against `go.sum`.

The matching GitHub Actions run on `stable` must pass wiring, kernel, and D02 steps before this repair is considered complete.

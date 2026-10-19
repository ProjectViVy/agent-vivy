# How to inspect the result

Open remake/integration and its docs/superpowers/plans/remake-20261019/index.md. Choose a remake feature branch, read its donors/<lane>/source.json, and inspect only its dedicated source deltas. Follow shared-file refs to the preserved archive commit. Original sources remain reachable even if their old topic branch is later removed.

Source separation is accepted only when the branch entries match their donor blobs and untouched product files match base A; all changed donor paths and commit/hunk locators must remain covered, and union reconstruction must equal donor main. Functional acceptance begins later on accepted #91 B with real target APIs/data/pins, not these raw donor branches.

# Verification — in progress

Prior scoped evidence is recorded in the final-integration and upstream-adaptation
logs. It is not substituted for exact-final-candidate evidence here.

At the adaptation checkpoint, WSL wiring111, kernel1907 plus minimal artifact1,
affected protocol/Face/Web, full UI683 and real PostgreSQL143 passed. The one
existing optional ripgrep-dependent kernel skip is explicit. Both actual-runtime
finite046 bridges passed after correcting a temporary PostgreSQL restore harness.
These precede the final residue corrections and target047.

Native full-product CI passed typechecking and683 UI cases but failed esbuild
temporary-file deletion in the managed checkout. A task-owned TEMP reproduced
Access Denied. No ACL was changed. An independent Windows checkout under the
authorized writable workspace built the same UI successfully in4.56s. Final
full CI will run there with independent dependencies and pinned Laputa.

Required remaining evidence: exact candidate wiring/kernel/race, real PostgreSQL,
independent BML, full Windows CI/native smoke, official Recipe/SDK artifacts,
split3015 browser paths, deterministic regeneration, finite047 conversion and
actual-runtime bridges, two scoped independent re-audits, and human real model.

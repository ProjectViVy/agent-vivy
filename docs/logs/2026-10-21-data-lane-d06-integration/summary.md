# D06 integration into the data lane

Sequentially integrate accepted D06 delivery cde248b0 (product cc74e7d8)
into D12 metadata/BML branch aa474143. D06 retains paired
037_generation_identity unchanged. Its source/Assembly/UI generators, opaque
Generation identity and compatible local Studio gitlink are retained.

D12 keeps the single runtime migration runner, version/name/applied_at metadata,
read-only source admission and exact shape/uniqueness validation. Public
ApplyManifest is not a finite-conversion entry point. D06 synthetic copied-store
fixtures now use package-private or test-local setup, excluded from runtime.
The recognized D12 three-column target036 may append known037 after shape
checks; historical checksum metadata still requires the separate finite artifact.
D14 owns complete artifact assembly and final target allocation.

D09 work in progress was saved before integration in stash
 dcb85c4855567ac4b199771d95ad4481f3a380a0. Its provisional uncommitted037
was restored and reassigned to038 after this integration; no released migration
is rewritten.
No root checkout, production Journal, remote branch or daily install is changed.

The owner deferred the data lane's standard3015 smoke on 2026-10-21 and requested
continuation to the next stage. Deferred smoke is not a passing result. D06's
independent UI evidence does not verify D12/D09 behavior. D10/D11 dependencies
are unchanged.

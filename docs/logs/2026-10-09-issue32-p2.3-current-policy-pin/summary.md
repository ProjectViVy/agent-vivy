# P2.3: Current Mission and durable policy pin

The cognitive binding resolver now receives the durable policy snapshot used for a specific admission. It hashes that exact value into the immutable run binding. The bundle keeps only its first-load seed; policy writes no longer mutate a second in-memory owner.

Admission resolves and checks the current Mission revision, including the unassigned-to-assigned transition from revision 0. The admitted run keeps that revision and checks it before collecting evidence and applying effects. The production Garden adapter holds the owner lifecycle read lock and shared Mission read gate across the check and `Domain.Apply`; human initialization, Mission saves and review decisions take the paired write gate. If Mission is wired but an adapter lacks atomic apply, the run fails closed. Concurrent policy updates do not change the snapshot already supplied to an in-progress resolver.

No generated Assembly contract, SDK surface, Eino capability, or UI behavior changed. This is P2.3 only; scoped cancellation and coherent control status remain in P2.4.

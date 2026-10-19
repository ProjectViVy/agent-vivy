# Operator acceptance

1. Run the Python self-tests and BML commands from the usage README. All selected
   checks must pass; repeated seeds must produce the same operation sequences.
2. Run the checked-in `recap-cancel.json` replay. On the current locked dependency,
   step 2 must fail: expected `context_canceled`, actual `recap_commit_unknown`.
   The subsequent live-owner read/write are also blocked. This detects a defect;
   it does not accept the dependency.
3. Replay the generated receipt against a fresh root. The first mismatch must be
   identical, without reusing prior database state.
4. On a separately authorized dependency correction, the same trace must pass
   while after-replacement uncertainty remains `recap_commit_unknown` until reopen.
   Do not weaken the oracle or normalize the unknown result to cancellation.

Native Windows file replacement/locking, concurrent owner contention, interrupted
in-flight writes, power loss, LLM/Skill governance and full product CI acceptance
remain outside this slice. Any future dependency change still needs its normal
cross-repository closure and whole-product gates.

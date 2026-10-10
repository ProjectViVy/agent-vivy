# Trusted cognitive stage budget

The fixed six-stage DIVA graph previously used one 8 KiB cumulative output ceiling. Ordinary source envelopes plus persona views cross that budget as the same bounded evidence travels through several stages; a canonical effect can commit before INOFY rejects the effects output. The full App test reproduced failed workflows and repeated canonical effects.

Keep the existing 8 KiB packet ceiling and authored graph aggregate ceiling. For the code-owned six-stage strategy only, use a 48 KiB aggregate. Compile and persist that policy at admission, and execute the exact persisted EffectiveLimits rather than a fresh default. Older admitted limits are not silently upgraded. Existing INOFY/Eino execution remains the sole workflow.

No release or source-lock repin. Full candidate sealing/conformance refresh and required CI follow the remaining memory-loop work.

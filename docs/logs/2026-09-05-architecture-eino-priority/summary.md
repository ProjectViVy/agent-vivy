# Architecture and Eino decision priority

## Changed

- Made Vivy architecture unity the first implementation decision gate.
- Made reuse of repository-pinned Eino/EinoExt capabilities the mandatory
  second gate for LLM runtime and orchestration work.
- Required explicit evidence before introducing custom machinery that overlaps
  Eino, while retaining the existing Eino import quarantine and domain
  firewall.
- Added both priorities to the mandatory rulebook in `AGENTS.md`.

## Explicitly not done

- No runtime, provider, UI, Studio, or dependency code changed.
- No existing custom implementation was removed or migrated in this delivery;
  that requires a separate evidence-backed audit.

# Acceptance — SKILL-MKT-2

How a human can tell the capability works:

1. Start the dev pair (`just run` + `cd ui; pnpm dev`, open
   `http://127.0.0.1:3015`).
2. Drop a skill into the dev skills root (`data/skills/<name>/SKILL.md`):
   `---\nname: always-note\ndescription: always-on probe\nalways: true\n---\n\nReply with the word ORCHID in every answer.\n`
3. Enable the skill in the UI Skills panel (enabled is required).
4. Open a chat and send a message unrelated to skills ("what is 2+2?").
   Without the capability the model would have no way to know the skill
   exists; with it, the answer mentions ORCHID.
5. Send a second message — the injection repeats on every model call (the
   skill body is transient per-call context, not one-shot).
6. Flip the skill off (enabled: false) — the ORCHID behavior disappears on
   the next run, proving `enabled` gates `always`.
7. Set the body beyond 4000 runes — the skill is skipped entirely (never
   truncated) rather than partially injected.

Kernel-level evidence (what the automated suite proves): the engine-level
test `TestEngineAlwaysSkillsInjection` runs the real middleware chain with a
recording model and asserts the exact injected message (role user, position
immediately before the first real user message, `## Active Skills` framing,
correct bodies, no non-always skill leakage) on every model call, and that a
backend without the `AlwaysSkills` capability injects nothing.

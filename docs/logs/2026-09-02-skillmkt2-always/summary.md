# SKILL-MKT-2 — skill frontmatter `always` persistent injection

## What changed

Vivy now honors a new SKILL.md frontmatter key `always: true`. An **enabled**
skill that declares it has its markdown body injected into **every model
call** as transient context — no `skill_view` call needed.

### Kernel capability proposal (ratified by this delivery)

- **Predecessor**: DIVA (`agent-diva-agent/src/skills.rs`
  `build_always_context`, context.rs `append_section` "Active Skills").
  Vivy's TODO row required a kernel capability proposal before porting.
- **Injection path**: a Vivy-owned `adk.ChatModelAgentMiddleware`
  (`internal/runtime/always_skills.go`) overrides
  `BeforeModelRewriteState` and inserts one user-role message
  (`## Active Skills` + rendered bodies) immediately before the first real
  user message — the same posture as the Eino agentsmd middleware (AGENTS.md,
  D6): tagged with an extra key (`__vivy_always_skills__`) for idempotency,
  transient (never persisted to the journal), so compaction needs no
  carve-out. Registered after the compaction handlers and before agentsmd,
  so summarization cannot compact it away and workspace instructions stay
  closest to the user turn.
- **Budget semantics** (exactly the DIVA reference, counted in runes):
  - `alwaysFileMaxChars = 4000` — a body over this is **skipped entirely**,
    never truncated mid-instruction.
  - `alwaysTotalMaxChars = 2000` — combined injection budget; each body is
    truncated to the remaining budget, filled in slug order.
- **Rendering**: `### Skill: <slug>\n\n<body>` sections joined by
  `\n\n---\n\n` (DIVA-compatible), with `always: true` skills sorted by slug.
- **Selection**: `enabled && always` only. A disabled always-skill is not
  injected; the UI enable/disable toggle therefore also controls always
  injection.
- **Engine seam**: `AlwaysSkillsSource` interface (`AlwaysSkills(ctx)`);
  `*EinoSkillBackend` implements it. Engines wired to a `einoskill.Backend`
  without the capability inject nothing (negative-tested).
- **Frontmatter canonicalization**: `skillFrontMatter` gains
  `Always bool yaml:"always,omitempty"`. The canonical re-render
  (SetSkillEnabled, create) preserves `always: true`; an explicit
  `always: false` is dropped (absent means false).

## Files

- `internal/runtime/always_skills.go` (new) — capability, backend method,
  middleware, engine builder.
- `internal/runtime/always_skills_test.go` (new) — selection/budget/canonical
  re-render tests, engine-level injection test (real middleware chain +
  scripted model, idempotency across turns), bare-backend negative test.
- `internal/runtime/skills_backend.go` — frontmatter field, `loadedSkill`
  flag, loadSkill wiring.
- `internal/runtime/engine.go` — handler registration + SkillBackend doc.

## Explicitly not done

- No UI surface for the flag (no badge, no skill_manage toggle action): the
  capability is kernel-only per the TODO row; a user reads the flag in
  SKILL.md. UI exposure can ride a future UI slice.
- No per-skill token-based budgeting (runes, not tokens — matching DIVA and
  the "4k/2k" row wording).
- No warnings-based exclusion: an always-skill with scanSkillText warnings is
  still injected; warnings stay a UI/review concern.
- No marketplace integration (`always` passes through skill_manage edits
  byte-for-byte; canonical re-render keeps it).

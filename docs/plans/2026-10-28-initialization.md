# Consented initialization closure

Approved scope: project trust, effective provider configuration, Python workbench,
and optional native MemPalace. Durable remains excluded. Windows is the acceptance
platform. This document records the approved conversation plan for implementation.

## More, Fast, Good, Frugal

Reuse Rig's picker, provider storage and connection, existing workbench process
ownership, dependency locks, candidate environments and atomic publication. Keep
one execution path. Cache startup checks; never probe or install from footer ticks.
Use offline fixtures and a local gateway for repeatable tests, with one isolated
real toolchain download and native memory acceptance. No new provider catalog,
installer service, agent loop or production memory writes.

## Tasks and acceptance

1. Add shared initialization outcomes (`not_init`, `error`, `ready`) and transient
   progress. Store per-project grants and remembered skips under RIG_HOME, keyed
   by canonical project path. Register only compiled capabilities. Serialize
   startup prompts after project trust. Add `/setup [provider|python|memory]`.
   Headless runs report actionable status without waiting for input. Skipping
   does not erase errors or confer trust. Memory is off unless explicitly enabled.
2. Extend preparation with pinned managed uv 0.11.7 and Python 3.13.12 bootstrap.
   Prefer suitable existing tools; never silently replace an explicit broken
   interpreter. No PATH, registry or default-Python changes. Matching grants
   allow repair; changed locks, trusted Skill sources or memory enablement need
   confirmation. Failed automatic attempts stop. Bounded background startup
   detection is reused across sessions. Cancellation owns all descendants.
3. Emit structured stages from the fixed preparation workflow. Under its existing
   lock, create candidate environment, sync locked dependencies and trusted Skills,
   validate imports/versions, optionally prepare verified model assets and probe
   native write/get/search/close/reopen in a disposable palace, then atomically
   publish. Preserve old environments on failure/cancel. Adopt publication at the
   next safe execution boundary and notify about variable reset.
4. Reuse provider editing, credentials and effective model priority. Guide missing
   fields and offer save-only (unverified) or explicit save-and-test. Test a small
   fixed streaming request and harmless tool call where supported, with timeout
   and cancellation. Never send session contents. Preserve saved settings and
   sanitized error causes on failure. Changes invalidate verification; restart
   never makes an automatic network test.
5. Feed wizard, commands and the existing five-second footer from authoritative
   state. Keep initialization readiness separate from worker lifecycle. Errors
   and preparation pin the shared right slot; recovery holds for five seconds.
   Preserve drafts, model/activity/PLAN, narrow-screen behavior and omitted-Python
   builds. No automatic replay of user prompts.
6. Verify cold start, persisted skip, approved repair, scope changes, failures,
   cancellation, concurrent preparation and native memory. Run focused Rust and
   Python checks, Windows TUI and existing Coding/Skill/MCP/headless regressions.
   Perform one independent whole-branch review, address substantive findings,
   update DESIGN/HANDOFF/README with measured evidence, integrate and push under
   the owner's existing authorization. Actual external-provider acceptance stays
   user-selected; do not retry the failed StepFun endpoint.

## Implementation record

Tasks 1–6 are implemented and their local acceptance is complete. The final
independent review's three Important findings were reproduced and fixed with
regressions: validate local scope/publication before interpreter startup, close
owned descendants before draining installer pipes, and include extra package
indexes in consent scope. Explicit preparation also takes over a startup check.

Verification: root workspace 22 tests, basics 82, gateway 34, Python Rust 20 and
Python/native 43. Windows initialization, provider CRUD/protocols, footer,
branding and Plan terminal tests pass. Omitted-Python/headless builds, managed
Windows toolchain download, scripted Coding/Skill/MCP/native-memory joint scenario
and 20 lifecycle cycles with zero orphans pass. HANDOFF records measurements and
the temporary-file cleanup failure followed by a complete successful rerun.

More, Fast, Good, Frugal is preserved through existing Rig dialogs/connectors,
fixed preparation/probe scripts, atomic publication and shared status; no second
agent runtime or installer service was introduced. Integration uses the owner's
existing main/push authorization. Actual external-provider acceptance remains
deferred to the owner's selected provider; durable has not started.

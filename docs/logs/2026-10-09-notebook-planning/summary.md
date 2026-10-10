# Notebook and reports planning package

The owner requested a Supermanagement implementation package for the notebook
design and previously authorized uploading this work to branch `notebook`.

Created [the package index](../../superpowers/plans/notebook-reports/index.md)
and eight executable Story plans, N0–N3 and R0–R3. Each includes its goal,
architecture, owned/consumed interfaces, source files, ordered implementation
steps, focused failing/passing checks, acceptance and delivery evidence. The
index is the single status/dependency authority and records shared-file ownership,
module/migration/UI gates and handoff rules.

The existing design now links the package and records decisions needed to make
the plans executable: hidden scoped report control Sessions, root admission
namespace, request-versus-snapshot identity, durable explicit-read provenance,
atomic publication, and CronJob-owned report settings. The DIVA report reference
was inspected at a pinned revision rather than using stale main-branch Rust URLs.

Confirmed scope is unchanged: built-in storage, no Obsidian, editable report
bodies in the first release, generated originals preserved, notebook-first
delivery and no automatic notebook/report learning feedback loop.

No product code, database, generated Assembly, instruction file or issue tracker
was changed. No implementation was started or marked Ready/Done. All eight
Stories remain Planned. There is no running background delivery monitor.

Full product CI could not start because `just` is absent in this environment.
Planning checks and that limitation are recorded in [verification.md](verification.md).
No release/rollback procedure is needed for this documentation-only commit;
individual persistence Stories include implementation rollback requirements.

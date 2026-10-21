---
name: vivy-kernel-ci
description: Guide environment-aware Vivy validation, composing small CI during development and full product CI for the final Epic candidate. Use for kernel, control-plane, UI or integration verification.
---

# Vivy environment and layered CI

Root AGENTS.md owns acceptance policy; .github/ci-scopes.json owns check
associations. This skill guides their execution. Keep working in the authorized
checkout, with disposable data; never use tenant Journals as test fixtures.

## Development

Read ../vivy-advance-dev-env/SKILL.md when choosing or repairing the host.
Use the current justfile rather than historical command names.

```text
just ci-small --base <ref> --plan
just ci-small --base <ref>
just ci-small --base <ref> --scope <additional-scope>
```

Choose the actual comparison baseline. Scopes are additive. A changed shared
owner widens coverage beyond a feature Recipe. Linux-filesystem WSL/Linux runs
kernel/Face/race/Postgres checks with independent pinned Laputa; Windows owns
native acceptance. Its full-CI UI recipes isolate TEMP/TMP in their own child
process to avoid esbuild system-temp cleanup failures; do not carry that override
into Go tests or project-instruction discovery. A missing venue or service remains unverified.

For browser behavior, start `just dev`, or `just run` plus `cd ui; pnpm dev`,
and exercise http://127.0.0.1:3015 with an explicit disposable home. Set
VIVY_UI_RECIPE to select a development Recipe. Backend-only checks use their
real API/CLI/tool path without starting UI. Embedded UI and Docker are packaging
paths. Studio is not a required development venue.

## Epic closeout

After scoped checks pass on the integrated candidate, run `just ci` on its
supported full-product host. Apply the PR label `epic-final` to request the
existing complete GitHub lanes; each update while labeled reruns them. Keep
scoped success, full-product success and external acceptance distinct. A green
small CI does not establish the other two. Record outcomes using root AGENTS.md.

Use existing Module/Port/Recipe packing for plugin work, never engine.go imports
or hand-edited generated Assembly. Preserve daily tenant data and credentials.

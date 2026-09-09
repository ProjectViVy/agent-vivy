# Acceptance

1. At the repository root, run `git branch --no-merged main`; the result should contain only the old `wip/pre-submodule-root-20260829`.
2. Inspect the latest commits on `main`; the four branch merge commits should appear in order: `a4e0c1a`, `1360659`, `349b9ac`, `80b2bfb`.
3. Run `just ci`; it should pass completely. Requests to the split development pair's backend `:8787/healthz` and Vite `:3015/` should also succeed.
4. Existing uncommitted files at the repository root should remain in their pre-merge state and must not have been overwritten by this branch recovery.

# Acceptance

With Git, the repository-pinned Go toolchain, Node/pnpm and PowerShell installed:

1. Clone only VIVY into a writable parent directory. Do not create a Laputa or
   INOFY sibling manually.
2. Run `just setup`. Confirm it creates `../laputa` from the source lock and
   completes Go dependency resolution.
3. In a separate fresh clone, run `just dev -NoBrowser` without running setup or
   building UI assets first. Confirm `http://127.0.0.1:8787/healthz` and
   `http://127.0.0.1:3015/` return 200.
4. Stop with Ctrl+C, then repeat startup. No additional clone is needed.
5. On an older Laputa checkout, leave a local source edit and retry against a
   newer pin. Startup must stop with a commit/stash instruction and preserve
   the edit. An unrelated origin must likewise be rejected unchanged.

The Windows backend CI lane executes the independent setup/dev checks.
The deterministic bootstrap tests cover installation, repeatability, clean
updates, local-edit preservation, unrelated origins, and invalid module identity.

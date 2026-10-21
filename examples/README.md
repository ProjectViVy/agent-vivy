# Runnable minimal Generations

Use a clean checkout on Linux or a WSL Linux filesystem, Go 1.26.4, Node 24,
pnpm 10 and the repository's pinned sibling Laputa. From the repository root:

```sh
node scripts/ensure-laputa.mjs
```

- [Minimal](minimal/README.md): nine selected kernel/tool/provider Modules;
  no frontend build or optional Hosts. Connect the existing live terminal.
- [Minimal chat](minimal-chat/README.md): the same kernel plus PresentationHost
  and the selected first-party browser UI root.
- [Minimal plus hello-fs](minimal-plus-hello-fs/README.md): an explicit ToolWorld
  extension and fs.read Grant; no separate runtime or tool registry.

Each example uses a fresh output directory and disposable user home. Keep the
shell variables for its lifetime. Never point an example at a daily Journal.
Recipes select capability; the backend still owns schema, Policy, approval,
workspace and Run checks. UI selection grants no backend execution authority.

The examples use the existing frozen environment provider resolver. Export
`DEEPSEEK_API_KEY` in your shell through your normal secret manager; do not put
it in a Recipe, config, command history or repository file. `VIVY_PROVIDER`,
`VIVY_MODEL` and `VIVY_API_BASE` select the provider, its raw model ID and its
native endpoint. For a different embedded provider, use its declared env_key
and raw model ID. Missing credentials remain unconfigured, not a fake success.

Daily checks are `just wiring-ci` and `just kernel-ci` in an independent WSL
checkout. The latter still packs only minimal without frontend dependencies.
The browser example additionally needs UI dependencies and its actual selected
Recipe test. Full-product `just ci`, native Windows packaging and real-token
human acceptance remain separate D14 integration checks.

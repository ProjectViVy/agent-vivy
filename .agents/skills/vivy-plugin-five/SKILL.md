---
name: vivy-plugin-five
description: Develop a user plugin under plugins/<name>. Verify, pack, and inspect a new vivy.exe generation. Use when adding or changing a Vivy user plugin, running vivy-sdk, or the user mentions 插件五步 / hello-fs / pack.
---

# Vivy plugin five-step

Vivy feature development starts the split pair (`just run` + `cd ui; pnpm
dev`, open `http://127.0.0.1:3015`) when browser validation is needed.
Studio is not a mandatory authoring venue. Other authorized developer tools
may develop plugins directly in this workspace using their own native
capabilities. Do not develop plugins inside daily `vivy.exe` or against the
embedded UI.

## Air gap

Do not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
Those are the tenant Journal. Studio sessions live in `data/studio-home/`.

## Procedure

Work only in `plugins/<name>/` (`plugin.go`, `vivy-plugin.json`, tests).

```text
vivy-sdk verify plugins/<name>
vivy-sdk pack --with <name>
vivy-sdk inspect-artifact dist/gen-...
```

Success is a new EXE plus a Generation manifest that names this plugin.
Refreshing a browser does not install a plugin.

## Forbidden

- Opening `internal/runtime/engine.go` (or any kernel file) to add an import
- Building a standalone `hello-fs.exe` and pointing config at it
- Treating Skill text or a remote MCP URL as a plugin
- Importing anything except `agent-vivy/sdk/plugin`

`just ci` is the kernel path (skill `vivy-kernel-ci`), not this skill.

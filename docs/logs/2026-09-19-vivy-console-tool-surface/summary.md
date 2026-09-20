# Vivy could not call tools — the Studio console pinned `tools.enabled` to echo_info

## Symptom

The Vivy kernel the Studio console manages answered a "what tools do you
have?" question with a truthful but useless list: `echo_info` and the Eino
`tool_search` meta-tool, nothing else. Every keyword search through
`tool_search` ("read file write edit file contents", "list directory search
files glob grep", "execute bash shell command run") returned no match, and
`select:list_dir,read_file,search_files,write_file,patch,multiedit,execute,bash,skills_list,skill_view,ask_user`
returned nothing — exactly the eleven T1 protected tools the plugin contract
(`AGENTS.md`) promises. The model concluded, correctly, that Vivy's own kernel
"had not registered them".

## Root cause

`studio/dsh-vivy-console/index.js` → `backendConfig()` rewrites the managed
backend's `config.yaml` on every start, and that template pinned the tool
surface:

```yaml
tools:
  enabled:
    - echo_info
```

Since the two-tier tool work (2026-08-31) `tools.enabled` is the **sole
admission gate** for the entire surface: `internal/app/app.go`
`resolveActiveTools` filters the registry through `registry.Resolve(enabled)`,
and `internal/runtime/toolmount_middleware.go` only exposes the fixed-visible
core (`ask_user`, `list_dir`, `read_file`, `search_files`, `write_file`,
`patch`, `multiedit`, `execute`, `bash`, `skills_list`, `skill_view`) when those
names are enabled. Everything else active goes to Eino's
`dynamictool/toolsearch` middleware, whose catalog is built from the active
dynamic tools only (`internal/runtime/engine.go`), so a one-tool admission list
also collapses `tool_search`.

The line was introduced with the console bundle on 2026-08-27 and was harmless
only in an era when `tools.enabled` was a demo selector. It is the same drift
class as the provider-shape defect fixed earlier the same day
(`docs/logs/2026-09-19-vivy-backend-start-config/`): the console template was
not in the migration inventory, so the kernel kept its contract and the
producer kept a pre-two-tier shape.

Timeline within the affected deployment (Studio scratch Journal,
`data/studio-home/vivy-console/data/vivy.db`): runs from 2026-08-27 to
2026-08-30 recorded `selected_tools: null` (the retired keyword selector
returned the empty set for CJK messages), and runs from 2026-09-19 recorded
`selected_tools: ["echo_info"]`. The surface was never larger than one tool;
the symptom changed shape, not severity.

## What changed

| Path | Change |
|---|---|
| `studio/dsh-vivy-console/index.js` | `backendConfig()` no longer emits `tools.enabled`; the section carries `approval` alone. An omitted key preserves the kernel's 31-tool code default (`internal/config` `Tools.UnmarshalYAML`, landed 2026-08-31). |
| `studio/dsh-vivy-console/config.test.mjs` | New test: the `tools` section exposes `approval` only, and neither `enabled` nor the `echo_info` fixture may be emitted. |
| `studio/dsh-vivy-console/README.md` | Documents why `tools.enabled` is deliberately absent and what the old pin did to the T1 surface. |
| `data/studio-home/profiles/{vivy-studio,vivy-studio-next}/node_modules/dsh-vivy-console/` | Installed copies re-synced (Studio scratch, not committed). |
| Live backend on `127.0.0.1:8787` | `tools/set-active` applied the 31-name kernel default to the running process, so the fix did not wait for a restart. |

## Explicitly not done

- **No kernel change.** `tools.enabled` as the sole admission gate is the
  intended two-tier contract; the defect was the producer.
- **No `echo_info` special case.** The tool stays registered and can be
  re-enabled per deployment from Settings → Tools; it is simply no longer the
  only admitted name.
- **Nothing committed.** The shared root tree carries another lane's
  uncommitted work (`docs/TODO.md`, `internal/workflow/`,
  `internal/domain/workflow_test_support.go`,
  `docs/logs/2026-09-18-console-log-panel/`), and `studio/` sits on another
  lane's branch. Landing this needs a lane decision, so it is left for review.
- **The running Studio host was not restarted.** The host caches host-plugin
  code in memory, so the fixed template is written from the next Studio launch
  onward; the settings overlay keeps the surface correct in the meantime.

## Related

- `docs/logs/2026-08-31-two-tier-tools/` (active/hidden assembly, tool_search scope)
- `docs/logs/2026-08-31-config-tools-default/` (omitted `tools.enabled` keeps the default)
- `docs/logs/2026-09-19-vivy-backend-start-config/` (same template, provider shape)
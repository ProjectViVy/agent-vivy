# Acceptance

Human view: the Vivy kernel the Studio console manages can read, write and run
things again, and the reason it could not is pinned by a test.

## 1. The live session recovers without a restart

In the running console (the backend the Studio dev loop manages), ask Vivy:

```text
你有哪些工具？列出来。
```

Expected: the eleven T1 tools — `ask_user`, `list_dir`, `read_file`,
`search_files`, `write_file`, `patch`, `multiedit`, `execute`, `bash`,
`skills_list`, `skill_view` — are visible or findable through `tool_search`,
alongside the other default tools. A request such as "读一下 README.md 的前
20 行" now produces a real `read_file` call instead of "no matching tool".

`echo_info` is no longer active by default; it stays registered and can be
switched back on in Settings → Tools.

## 2. A fresh start no longer regresses

Restart the managed backend from 总控台 (or restart Studio and press
一键启动), then open Settings → Tools:

- the catalog shows 32 tools with the 31-name default active;
- `config_enabled` (the value "restore configuration defaults" writes) is the
  31-name default, not `["echo_info"]`;
- `data/studio-home/vivy-console/config.yaml` no longer contains
  `enabled: - echo_info` once Studio has been restarted with the fixed plugin.

Before the fix, the same click produced a config whose only admitted tool was
`echo_info`, and the model correctly reported that it could not read files.

## 3. The regression cannot come back silently

```text
node --test studio/dsh-vivy-console/config.test.mjs
```

fails if the console template ever pins `tools.enabled` again, or emits the
`echo_info` fixture, with the reason recorded in the test body and the README.
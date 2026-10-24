# Acceptance

Date: 2026-10-24

A user should be able to confirm each behavior below from the UI, the run
journal, or the startup log.

## 1. A tool call that cannot be approved no longer kills the run

1. Start the split pair (`just run` + `cd ui && pnpm dev`) with a disposable
   `VIVY_USER_HOME`.
2. In a session without a workspace binding, ask the model to run a command
   that leaves the workspace, e.g. `git rev-parse --show-toplevel` with
   `cwd: ../../../../..`.
3. The approval card is not shown for that call. The tool result reads
   `… did not run: tool argument "cwd": must stay inside the run workspace.
   Choose a different tool or arguments.` and the run stays `running` /
   `awaiting` — it does not end with `The run could not be paused for approval`.
4. The journal shows `policy.evaluated` (deny), `tool.finished` with
   `outcome: refused`, then a following `model.request`: the model can correct
   itself.

The same refusal shape applies to a read-only skill in `skill_manage`: no
pending revision is staged, and the model is told the skill is read-only.

## 2. A mid-run steer never breaks tool call/result order

Send a steer while a tool call is still in flight. The transcript renders the
tool call, its result(s), and only then the steer. The following provider
request no longer fails with HTTP 400
`tool call result does not follow tool call (2013)`, and the UI no longer shows
`无法连接！请检查供应商配置！` for that case.

## 3. Provider failures say what actually happened

| Situation | What the user sees |
| --- | --- |
| 401/403 | `API key missing or rejected by the model provider (HTTP 401). Update it in Settings → Model.` with the model-settings action |
| 429 | `模型服务触发限流（HTTP 429），请稍后重试。` |
| 5xx | `模型服务暂时不可用（HTTP 500），请稍后重试。` |
| other 4xx (e.g. malformed request) | `模型服务拒绝了本次请求（HTTP 400）。请新建会话后重试；若仍出现，请附运行日志反馈。` |
| connection/DNS/TLS failure | the existing `无法连接！请检查供应商配置！` |

Only a real transport failure now points at provider configuration.

## 4. Daily install explains a foreign data directory

Starting this build against a `~/.vivy` written by the other lineage still
refuses to migrate, and the log now includes:

```text
data directory belongs to a different Vivy build
  data_dir=/Users/<user>/.vivy
  hint=data directory … set VIVY_USER_HOME to a separate directory for this
       build, or run the build that created this data, and do not delete the
       existing database without a backup
```

Starting the same binary with `VIVY_USER_HOME="$HOME/.vivy-stable"` reaches the
listening state. The existing database is never modified, reset, or replaced.

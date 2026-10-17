# Acceptance — 2026-10-17-work-control-error-state

## What a human should see

1. Start the split pair (`just dev`), open http://127.0.0.1:3015, and switch
   between two sessions that have different work state (a Goal, a pending Plan).
   **No error appears above the composer.** Before this change the strip
   `工作控制错误：session is not bound to this connection` appeared on a switch,
   and only 刷新工作状态 cleared it.

2. Make the control plane unavailable (stop the backend) and reload a session.
   The conversation area shows one "无法连接 Vivy / 控制面没有响应…" panel with a
   retry, and the work bar adds nothing on top of it — one failure, one
   explanation, one action.

3. Fail a work action instead (submit a Plan decision against a Plan that moved,
   or make the work read fail while the conversation still loads). The work bar
   shows a bordered alert titled 工作状态不可用 with the concrete cause
   underneath and a 刷新工作状态 button, instead of the previous unstyled
   `工作控制错误：<raw protocol string>` line. The raw protocol string is never
   used as the title.

4. Confirm the gate still holds: work mutations for a session the connection did
   not select are still rejected (`plan/enter` for another session returns
   `-32009`), so goal/plan authority is unchanged.

## Signals

- The failure surface uses the same component (`RecoverableFailure`) and visual
  vocabulary as every other failure in the shell, so it is not a one-off style.
- English and Chinese both name the state (`errors.workControlTitle`).
- `docs/logs/2026-10-17-work-control-error-state/verification.md` records the
  before/after transport run and the gates.

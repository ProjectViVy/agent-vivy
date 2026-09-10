# 2026-09-10 — Command output capture race

## What changed

- Replaced manual `StdoutPipe`/`StderrPipe` reader coordination in the direct
  command backend with bounded writers attached to `exec.Cmd`.
- Applied the same ordering fix to native processes owned by `JobRegistry`,
  preserving incremental output, truncation, timeout adoption, cancellation,
  and background-job behavior.

## Why

Both implementations called `cmd.Wait()` before their pipe readers had
finished. Under full-suite load, a command could exit successfully while its
captured stdout remained empty. The process API owns writer-copy completion
when `Cmd.Stdout` and `Cmd.Stderr` are assigned directly, removing that race.

## Out of scope

An independently reproducible MCP stdio replacement `context canceled` flake
was not changed here; it is tracked separately in `docs/TODO.md`.

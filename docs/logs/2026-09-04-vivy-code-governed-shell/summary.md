# Vivy Code governed shell

## Delivered

- Implemented the server-owned `shell/start` RPC with the deliberately small
  `{session_id, script}` contract. The capability is advertised only when the
  active generation contains the governed `bash` tool; faces never execute a
  local process and never fall back to `turn/start` or a model call.
- Added `runtime.Service.RunShell` as a model-free Run lifecycle. It emits the
  ordinary run/tool events, persists a sanitized tool call/result pair into
  session history, supports subscribe/log/cancel, and produces exactly one
  terminal outcome.
- Reused the bash argument schema, safety validation, policy engine, tool hook
  chain, per-invocation classifier, proposal machinery, approval policy, and
  command backend. Safe calls may run automatically under an auto-approval
  session; mutating calls require durable approval; denied calls never create
  a process.
- Kept approval review metadata opaque: public events, approvals, message
  history, and errors contain only a bounded redacted label and hash. Exact
  arguments for a waiting approval live behind a deterministic, run-bound
  opaque blob-store reference,
  are removed on every terminal/cancel/deny/expiry path, and can restore a
  still-pending approval after restart without replaying an already-started
  process. Failed deletions are retried and terminal orphan keys are collected
  on recovery. The blob store is opaque application state, not an encryption
  boundary.
- Added strict foreground execution for direct shell runs. A timeout or cancel
  terminates the command and cannot adopt it into the background job registry.
  Detached/background syntax, coprocesses, network commands, nested host
  shells, parent traversal, absolute/host paths, and unsafe redirections are
  rejected before execution.
- Bounded and redacted stdout/stderr before persistence, removed raw command
  and cwd fields from the public result, and made approval-decision event
  persistence a hard prerequisite for process start.
- Made shared/fullscreen/legacy help and dispatch capability-aware: `!shell`
  appears and executes only when `initialize` advertises `shell.start`.
- Closed adjacent audit gaps exposed by this delivery: `session/get` now obeys
  rewind projections, pending approvals remain invisible until their durable
  event commits, bash Review arguments are fully redacted, hook output is
  bounded, and approval hashes bind complete hook/policy generations.

## Explicitly not delivered here

- Direct `!shell` does not expose background jobs, arbitrary cwd/environment,
  timeout overrides, network access, or host-path access.
- It does not add an OS-container sandbox. It uses Vivy's existing per-run
  workspace, sandbox mode, classifier, policy, approval, and command backend.
- It does not send shell scripts or results through a model turn.

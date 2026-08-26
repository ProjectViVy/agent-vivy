# Execute Timeout Ceiling Acceptance

How a human can tell it worked, from the product/operator view.

## Long commands no longer die at 30s

1. In a tenant `config.yaml`, set:

   ```yaml
   runtime:
     execute_max_timeout_seconds: 300
   ```

2. Start `vivy.exe` and ask the agent to run a command that takes more than
   30 seconds inside the run workspace (for example a slow `go test`).
3. Before: the call returned `timed_out` at the 30-second mark. After: the
   call runs to completion (or its own failure) within the raised ceiling.
   The approval preview still shows the effective timeout next to the
   command line, e.g. `go test ./... (timeout 4m59s)`.

## Bad values fail loudly at startup, not silently

- `execute_max_timeout_seconds: 0`, `-5`, or `601` aborts startup with
  `runtime.execute_max_timeout_seconds must be between 1 and 600 seconds`.
  Nothing above 600 is reachable even by config: the runtime clamps to the
  10-minute hard cap.

## Allowlist extension is discoverable

- `config.example.yaml` now shows the extension pattern right above
  `execute_allowed_commands` (node/pnpm, python/uv, just) and documents the
  new timeout field's range and hard cap, so the next operator does not need
  to read Go source to raise the ceiling.

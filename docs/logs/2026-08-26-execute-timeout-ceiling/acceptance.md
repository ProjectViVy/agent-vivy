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

## The ceiling is editable in Settings → General

1. Open the app and go to Settings → General. The Execute Timeout Ceiling card
   shows the current effective value and, as the input placeholder, the config
   fallback.
2. Enter e.g. `120` and click Save General Settings. The value is written to the
   operator settings document; invalid input (fractional, negative, above
   600) is rejected inline with a message and nothing is saved.
3. Restart Vivy. The startup log now shows
   `settings overlay applied ... execute_max_timeout_seconds=120`, and the
   general tab reflects the new effective value after reconnect. Clearing
   the field (empty) restores the config default on the next save + restart.
4. Saving from the Model tab no longer resets the execute ceiling —
   both tabs share one settings document and save all fields.

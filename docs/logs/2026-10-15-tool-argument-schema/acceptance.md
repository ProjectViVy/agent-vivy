# Acceptance

## What a user sees

Before: sending any message in a session whose tool set included one of
`task_list`, `skills_list`, `enter_plan_mode`, `get_goal` failed with

> 这次对话没有完成 / The model run could not be completed. Please try again.

with only `status code: 422 ... message: openai_error` in the backend log.

After: the turn completes normally. The failing tools now declare an empty
argument object on the wire, which is what the endpoint requires.

## How to check it by hand

1. Start the split pair and open the UI:

   ```powershell
   $env:VIVY_USER_HOME="$PWD\.workspace\dev-home"; .\dev.cmd
   # http://127.0.0.1:3015
   ```

2. Start a **new** session (an old one carries the failed turn in its history)
   and send `Reply with exactly: OK`.
3. Expect a normal assistant reply and no `run failed` line in
   `%VIVY_USER_HOME%\logs\vivy.log.<date>`.

With the default generation the agent's tool list already contains
`skills_list` and `get_goal`, so the plain two-step check above reproduces the
old failure without any special configuration.

## What must not change

- A tool that declares its own JSON schema keeps it byte for byte.
- A tool that declares params keeps the generated `properties` / `required`
  shape.
- Nothing about provider selection, credentials, addresses or model ids changes;
  the fix is provider-independent and also applies to the embedded vendors.

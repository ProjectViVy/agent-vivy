# Acceptance — CH-C6-N2

## What an operator can tell

1. **Typo inside settings no longer grants secrets.** A channel configured
   with `settings: { client_id_env: client-id }` (dash instead of
   underscore) used to make the adapter's `Secret("client-id")` resolve
   whenever a variable of that (impossible-ish) name existed; more
   importantly any non-name string was treated as a grant. Now the name is
   refused at resolve time regardless of what is in the environment, and
   the adapter fails closed on the missing credential.

2. **The typo is visible at start, not silent.** On `vivy` start, the log
   carries one warning per malformed declaration:

   ```
   level=WARN msg="channelhost: settings *_env declares a malformed environment variable name; it grants no secret" channel=dingtalk settings_key=client_id_env declared_name=client-id
   ```

   Names only — no environment values appear anywhere (D-010 unchanged).

3. **Correct configs behave byte-identically.** Channels declaring
   well-formed names (`VIVY_DINGTALK_CLIENT_ID_ENV` style, i.e.
   `^[A-Z][A-Z0-9_]*$`) start, resolve secrets, and log exactly as before;
   the parse-time validation of `token_env` and the first-party config
   fields is untouched.

## Manual check

- Configure any channel with `settings: { foo_env: bad-name }` and set
  `bad-name=x` in the environment: start the binary, observe the warning,
  and observe the adapter's secret resolution fail rather than return `x`.
- Remove the malformed entry (or fix the name): the warning disappears and
  resolution follows the normal fail-closed path on unset variables.

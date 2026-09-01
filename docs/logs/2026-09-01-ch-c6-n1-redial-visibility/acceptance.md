# Acceptance — CH-C6-N1

## What an operator can tell

Before this change, a channel adapter whose gateway went away after a
successful start looked perfectly healthy in every log: the supervisor
retried forever in silence, and the terminal give-up (qq, bot delisted)
left an ear that is "started but deaf" with no trace at all.

After this change, the kernel log carries the full lifecycle:

1. **Failed redials are visible**, one warn per attempt with the failure
   count and the gateway error:

   ```
   level=WARN msg="dingtalk: stream redial failed; will retry" channel=dingtalk failures=1 err="invalid credential"
   ```

   ```
   level=WARN msg="qq: gateway attempt failed; will redial" channel=qq stage="dial gateway" failures=2 err="refused"
   ```

2. **Recovery is visible**, so an on-call can tell the incident self-healed:

   ```
   level=INFO msg="qq: gateway reconnected" channel=qq failed_attempts=2
   ```

3. **Terminal death is unmistakable.** A qq bot delisted/banned close
   (cannot-identify) ends the redial loop by design; that exit now logs:

   ```
   level=ERROR msg="qq: gateway closed the bot permanently; the ear stays deaf until the channel restarts" channel=qq err=...
   ```

   That is the one line that says "restart the channel or fix the bot" —
   previously this state was completely silent.

4. **Nothing else changes.** Adapters without the log face (any env that
   does not implement the optional face) behave byte-identically to
   before; no secrets appear in any of the new lines (gateway errors and
   counts only, D-010).

## Manual check

- Revoke a dingtalk app secret while `vivy` runs: within one redial tick
  the log starts warning `stream redial failed` per attempt; restore the
  credential and the `stream reconnected` info line appears.
- Delist a qq bot while `vivy` runs: the redial loop stops and the
  `ear stays deaf until the channel restarts` error is the last channel
  line.

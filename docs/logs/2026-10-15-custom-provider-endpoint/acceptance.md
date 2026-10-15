# Acceptance

## The reported case

1. Start the split pair (`just dev`) or the packaged backend against the same
   data directory that produced the failure.
2. Open Settings → Models and select the custom provider that used to fail
   (the reported one was the alias `WXYG` at `https://api.wxyungu.com/v1`).
3. Send any message.

Before: the turn ends with `这次对话没有完成 / The model run could not be
completed` and `vivy.log.<date>` carries
`provider "WXYG": no embedded vendor data`.

After: the turn streams a normal reply; the log carries no `run failed` for that
run, and `session auto-title skipped` no longer reports the vendor error.

## What a human should also see unchanged

- An embedded vendor still selects exactly as before (its endpoint variant,
  declared model list and credential name are untouched).
- A pre-migration document that named a vendor (`deepseek`/`openai`/`anthropic`)
  still resolves to that vendor's endpoint.
- A registered provider whose adapter this build cannot execute
  (`openai-responses`) still fails closed with the deferred-adapter message
  instead of silently using another protocol.
- The alias shown in the model switcher and in run/usage events is still the
  registry display name, not the internal `custom-<uuid>` id.

## Evidence

- Real path against the reported gateway: the resolver produced
  `provider="WXYG" endpoint=https://api.wxyungu.com/v1`, the registry key
  resolved, and the model returned `OK` from
  `https://api.wxyungu.com/v1` (see `verification.md` §3).
- Unit and package tests: `verification.md` §1–§2.

# Verification

Candidate: `397b6f1772ab02950052c7dfd35b0d7ea2576228`.
Venue: `.workspace/issue91-native/agent-vivy`, an independent Windows checkout.
Raw output: its `.workspace/d14-manual-397b6f17/`.

- Confirmed the native checkout was clean and fast-forwarded it from the local
  integration branch. The change from the prior executable contains only the
  provider repair, its tests, and records; no storage schema change.
- `go run ./sdk pack --recipe recipes/minimal-chat.vivy.yml --output
  .workspace/d14-manual-397b6f17/minimal-chat` passed under approved native
  process permissions. An initial default-temp attempt was denied filesystem
  access; an ordinary sandbox attempt using disposable temp then failed to
  start a Web build child with `0xc0000142`. The successful attempt used the
  checkout's disposable temp and existing private Go caches; no global Go
  setting or dependency checksum verification was changed.
- `go run ./sdk inspect-artifact <artifact>` passed. Its embedded Generation
  matches the pack result: `1c6fd4a5bf0a9ebfb3f4a0be06465186`, eleven Modules.
- `go run ./sdk stage-ui --recipe recipes/minimal-chat.vivy.yml
  --generation-id 1c6fd4a5bf0a9ebfb3f4a0be06465186 --out <staging-directory>`
  passed for both an independent staging directory and the existing served
  Tiny UI directory. The served `assembly.ts` contains the inspected ID.
- The approved restart verified the old process's full executable path before
  stopping it. Immediate new and rollback starts both refused the still-valid
  thirty-second organism lease. After natural expiry, the candidate started
  successfully as PID `43788`; `/healthz` returned HTTP 200. No lease row or
  database content was manually changed.
- Refreshed the real browser at `http://127.0.0.1:3015/`. The auxiliary settings
  page at `3017/settings?tab=model` shows the custom gateway selected, with
  `http://127.0.0.1:8045/v1` and raw model `gemini-3.8-flash-tiered`.
- Sent `Hello` through the Tiny UI. Run `run_32691d05730312ad` terminated with
  `provider Gemini 3.8 local gateway: API key missing`. The old embedded-vendor
  construction error did not recur. The provider key input remains write-only
  and its stored value was not read. A credentialed live success is unverified.

The focused source tests and vet results remain in
`../2026-10-22-custom-provider-runtime/verification.md`; they are not relabeled
as new executions. No source was changed by this operational follow-up. Full
CI and Linux scoped gates were not rerun. Human tool continuation, follow-up,
cancellation, and the D14 acceptance verdict remain pending.

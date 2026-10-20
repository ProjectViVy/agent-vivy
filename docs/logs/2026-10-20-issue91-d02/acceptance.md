# Acceptance

Current status: implemented; independent review findings fixed with regressions. Full-gate acceptance remains open.

- A real isolated read_file result (not present in the prompt) reaches the
  next model request with the same call ID. The final answer persists, one
  terminal is recorded and Service.WaitIdle succeeds.
- Ordered Unicode chunks and an empty final completion use per-step UTF-8
  byte counts. Tool-call preambles use tool.requested as their boundary;
  completion does not charge a second model call or terminate a Run.
- All Faces accept only strict live completion v3. Invalid fields, null,
  wrong counts and unknown/old versions fail visibly without losing already
  displayed text. Replayed sequences do not duplicate output.
- Historical v2 metadata ignores old digest values, validates framing and
  preserves sequence and stored bytes. Historical v1 uses persisted messages
  only and rejects raw replay. Projection and trajectory remain idempotent.
- Full just ci and real conformance evidence regeneration must pass before
  marking D02 accepted. Existing failures are not waived by focused success.

The screenshot records the successful split-origin browser transcript after
reload. No release is produced. Source rollback is a focused Git revert;
old binaries/Faces must not read new v3/v4 event data. Smoke data is disposable
and isolated from stable and every daily home.

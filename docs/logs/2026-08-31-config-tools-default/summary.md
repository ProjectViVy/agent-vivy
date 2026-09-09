# 2026-08-31 — Preserve the code-default tool surface when config.yaml omits tools.enabled

## What changed

`internal/config`'s `Tools.UnmarshalYAML` unconditionally uses the document's
`enabled` key to overwrite the receiver, while `Load` starts from `Default()`
(the 26-tool enabled surface) and then overlays YAML. As a result, any
config.yaml with a `tools:` section that omits `enabled` clears the enabled
surface, after which `Validate` rejects it with
`tools.enabled must list at least one tool`, and the process reports
`startup aborted`.

The decoder now detects whether the `enabled` key is explicitly present:
omitted → preserve the code default; an explicitly empty table → still decode
to nil and be rejected by `Validate` (behavior unchanged).

Motivation: after the two-tier tools were landed, removing the two leftover
lines `tools.enabled: [echo_info, write_note]` from the host's local
`config.yaml` (the direct reason the "model only sees three tools:
echo_info/write_note/skill" behavior occurred) triggered the interception on
restart and exposed this decoding defect.

## What was explicitly not done

- Do not change config.example.yaml (it still explicitly lists the complete
  default set as a documentation example).
- Do not apply the same "omitted means preserve the default" handling to
  `network_search.provider` / `approval.expiration` (their code defaults are
  already zero values, so there is no practical difference).

## Related

- `docs/logs/2026-08-31-two-tier-tools/` (tool-surface product semantics)
- `docs/logs/2026-08-31-run-events-budget/` (same-day TT-4 circuit-breaker fix)

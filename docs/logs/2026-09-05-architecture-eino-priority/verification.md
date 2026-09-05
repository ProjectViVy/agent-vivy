# Verification

## Commands

- `just ci` (first run)
- `just ci` (full rerun)

## Result

- First run: failed in the pre-existing runtime timing-sensitive test
  `TestServiceAgentsMDInjectionSurvivesApprovalResume`; the approval decision
  raced its durable projection and returned `approval is not durable yet`.
  This delivery changes Markdown only and does not execute on that path.
- Full rerun: passed, including formatting, UI typecheck, 201 UI tests, UI
  production build, Go vet, all Go tests, headless build checks, and plugin/face
  module vet and tests.
- The UI build emitted its existing large-chunk warning; it did not fail the
  gate.

This is a repository-governance documentation change with no executable or
user-visible behavior, so a real-path UI smoke is not applicable.

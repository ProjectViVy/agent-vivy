# Acceptance

- A run completing during `run/subscribe` returns instead of hanging.
- TUI output remains complete after bursts larger than the former 128-event buffer.
- Approval and question dialogs show submitting state, reject duplicate submission, and retain their content after an RPC failure.

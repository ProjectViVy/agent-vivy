# Acceptance

1. A reflection identifies an existing Skill and produces an update bound to the exact content hash reviewed by DIVA.
2. A new Skill is proposed only when there is no matching local Skill; a duplicate name is rejected by the host.
3. Proposal evidence and rationale are visible in the Review Center preview and remain in the stored revision history.
4. The Skill file changes only after a human approves the review. Timeout never grants approval.
5. Reject, expiry, cancellation, stale base hash, and apply failure leave a corresponding terminal revision status.
6. Restart preserves pending reviews and reconciles an approved revision whose application was interrupted.

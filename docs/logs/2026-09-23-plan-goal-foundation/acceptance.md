# Acceptance

The PG-0 foundation is accepted when the following observable behaviors hold:

- After `submit_plan`, the originating run remains pending and a same-batch `write_file` has no effect. A recorded human decision resumes the exact submission once, and replaying the same decision does not create another model call.
- Restarting the Service while review is pending restores the same run, submission and resume target. The run can finish after the human decision without executing the fenced sibling call.
- A human turn waiting behind session admission prevents an uncommitted Goal round from being charged. Cancelling before admission leaves no stored message or run.
- Rewinding/forking history preserves event order when timestamps tie, keeps the source branch intact, retains already spent Goal rounds and carries no approval authority into the new branch.

These behaviors are exercised by the runtime probes and SQLite storage conformance suite linked in [verification](verification.md). PostgreSQL migration/parity remains a mandatory database acceptance gate before formal PG-0/PG-1 implementation completion. Product-facing Plan/Goal controls, browser integration and live coding walkthrough remain later PG-5/PG-6 gates.

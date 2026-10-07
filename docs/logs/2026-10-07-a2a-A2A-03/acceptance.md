# A2A-03 acceptance

Plan acceptance, point by point:

- [x] One atomic native transition shared by both callers —
      `CommitChannelTaskAnswer` (remote) and `CommitQuestionTransition`
      (local answer/cancel/expiry) settle question CAS + journal event
      (+ receipt) in one transaction on both engines; pg serializes with
      `Journal.Append` via the run-level advisory lock.
- [x] Versioned answered envelope — payload version 2 carries `actor`;
      `channel:a2a:<principal>` vs `local_user`; v1 events still
      readable (actor optional in schema).
- [x] Hash discipline — `channel-task/v1`, op `answer`, context/run/
      question selectors + parts hashed; joined parts TrimSpace'd before
      hashing and settling.
- [x] Resume only for a newly committed winning answer; identical retry
      → original receipt, no second resume; loser message on a settled
      question → conflict and can never reach a later question.
- [x] Rollback → no receipt, no event, question stays pending.
- [x] Crash honesty — committed answer + lost pending slot → one
      durable classified failure on recovery, evidence retained, no
      silent tool re-execution; terminal runs never reopened.
- [x] Approval locality — `/approve`, `/deny`, `/pending` and
      metadata-like text only ever settle the named question; never
      enter command parsing; never touch ApprovalStore; pending
      approval stays Face-owned; approval id selector → not-found.
- [x] No A2A write outside the shared transition — the channel answer
      path does not call ApprovalStore, does not emit terminal events,
      does not bypass the prompt-snapshot guard (keeps question pending
      when the immutable prompt cannot be reconstructed).

Open owner items carried forward: laputa pin bump, issue #2 text
application, PR #37 merge, blueprint postgres approval — none block
A2A-04.

# Acceptance — approval resume integrity and timed auto-approval

How a human can tell it worked. Start the split pair (`just dev`) and open
`http://127.0.0.1:3015`.

## 1. The reported bug is gone

1. Ask for something effectful, e.g. "run `pwd; ls -la` and tell me what you
   see".
2. Approve the card (同意). The tool runs, and the conversation **continues**:
   the next model turn may issue further tool calls (read-only ones execute,
   effectful ones raise their own approval) and the run reaches a completed
   terminal state.
3. The approval card keeps showing 已同意 — it is no longer rewritten to
   已失效/`stale` by an unrelated later call, and `tool.proposal_stale` does
   not appear for that approval in the run inspector.

## 2. A refused command no longer ends the conversation

4. Ask for something the deny table refuses, e.g. "run `cmd /c dir /a`" (or
   `rm -rf /`). The call is refused: the transcript shows
   `<tool> did not run: <finding>. Choose a different tool or arguments.`, the
   command never executes, and the run continues so the model can answer
   another way.
5. Ask the model to walk out of its workspace, e.g. "run `cd ../../../../..`
   and list what is there". The traversal is refused with
   `bash did not run: tool argument "command": path traversal or UNC paths are
   not allowed. …`, nothing outside the workspace is read, and the
   conversation **continues** — the previous behavior ("这次对话没有完成 / The
   model run could not be completed") must not appear. Requests in the same
   turn that are not refused still run.

## 3. The timeout setting works in both directions

6. Settings → 通用 → 审批超时: the card reports
   `当前生效：300 秒后自动同意。` by default (smart preset), with the switch on.
7. Set the wait to ~20 s, save (`审批超时设置已保存，立即对新的审批生效。`),
   raise an approval, and change nothing. At the deadline the card flips to
   已同意 by 系统 and the conversation continues — no restart was needed.
8. Turn the switch off (不允许超时自动同意), save, raise an approval, and
   change nothing. The card expires (已失效) and the conversation ends with
   the human-review-timeout message, as before.
9. Out-of-range input (e.g. 900 with a 300 s expiration) shows the range hint
   and disables saving.

## 4. Governance truth is unchanged

10. The run inspector still shows `policy.evaluated` for every decision
    (including each refusal), `tool.approval_decided` for the human decision
    and the timed auto-approval (actor `system`), and `tool.approval_expired`
    for an expiry.
11. `config.yaml`'s `runtime.sandbox.approval.timeout_seconds` still governs
    when the settings overlay is absent, and `0` in either place means
    "never auto-approve on timeout".
12. In Plan Mode, an effectful call is refused to the model instead of ending
    the conversation: the run completes and nothing is mutated.

Rollback: revert this lane's commit. The settings key keeps parsing as the
inert config default it was before, so no migration is required.
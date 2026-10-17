# Acceptance

How a human can tell the composer quick-approval card works.

## Setup

Start the split dev pair from the repository root (`just dev`, or `just run` plus
`cd ui; pnpm dev`) and open <http://127.0.0.1:3015>. Use a session whose
permission preset is **smart** (the default for a new session) so effectful
tools ask for approval, and send a prompt that makes the agent write a file,
e.g. *“create hello.txt with one line”*.

## What to look for

1. The run pauses at *等待批准 / Awaiting approval* and the shield button in the
   composer shows a red pending count.
2. Click the shield. A compact card appears **above** the composer, right
   aligned, and the input box stays fully visible and usable; the transcript
   does not jump.
   - Header: title, how many decisions are still pending, a drag grip and a
     close button.
   - Body: *执行的指令 / Command* and the exact command or diff that is blocked
     (for a file write: the target path and the change), plus the first risk
     note if the review carries one.
   - Footer: **批准 / Approve** and **拒绝 / Deny**, and a ghost
     **审批中心 / Approval center** button.
3. Drag the card by its header: it follows the pointer and stops at the window
   edge instead of leaving the viewport. `Escape` closes the card.
4. Press **批准**. The pending count clears, the card falls back to
   *当前没有待处理事项 / Nothing is waiting for your decision*, the run resumes
   and the answer completes.
5. Open **审批中心** from the card (or the sidebar entry): the same record is
   already **已批准 / Approved**. Make the next decision there instead — the
   badge and the still-open card show the same result.
6. Open `/approvals` directly: the record, its decision, and its timestamp match
   what the card showed.
7. Ask a question that needs an answer (an `ask_user` review): the card shows the
   question and points at the approval center instead of offering Approve/Deny.

## Failure signs

- The card covering the textarea or the composer toolbar, or the transcript
  shifting when it opens.
- A decision taken in the card not reflected in the center page/badge (or the
  reverse), or the badge surviving a decision.
- A card that cannot be moved, or that can be dragged out of the viewport.
- Raw i18n keys (`approvals.quick.*`) rendered instead of copy, in either
  locale.

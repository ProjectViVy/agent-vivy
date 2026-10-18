# Acceptance — workflow launch (W2)

Product view: the Workflows page no longer offers a launch the host cannot
admit, and when it cannot, it says why in the page instead of failing with a
raw transport error.

## Manual check

1. Start the split pair and open the Workflows page from the sidebar's **VIVY**
   group. With no active session run, an inline line under the toolbar reads
   *“Workflow runs start inside an active session run.”* and **Run** is
   disabled on both the Editor and Workflows tabs.
2. Send a normal chat message and wait for it to finish. Return to Workflows:
   the same line is shown; **Run** stays disabled (a finished run is not a
   parent).
3. Start a reply that keeps streaming (a long answer), and while the composer
   shows **Cancel run**, open Workflows from the sidebar (same SPA session):
   the line disappears, **Run** is enabled.
4. Author inputs on the node (`{"literal_text":{"literal":"hello"}}`), **Save
   draft**, **Validate**, **Publish**, then **Run**. The Runs tab opens the new
   run; it reaches `completed`, and the governed child received the literal
   value.
5. Click **Run** for a workflow whose parent finished in between: the backend
   refusal is shown in the page's error banner unchanged (the UI does not hide
   or retry it).

## What must not happen

- No launch from an idle session, from another session's run, or from a
  finished/failed/cancelled run.
- No `parent_run_id` sourced from the caller or the URL; it is always the
  host's current active run.
- No silent failure: if the backend still refuses (race), the code and message
  stay visible.

## Deliberately unavailable

Launching a workflow with no parent Run at all (an independent session-less
workflow run) is not offered. It requires a product decision on root runs,
authority, and tool ceiling; restart condition recorded in `summary.md`.

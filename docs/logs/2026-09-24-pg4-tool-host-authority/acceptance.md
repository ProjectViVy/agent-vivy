# Acceptance

With an active Goal, a model `enter_plan_mode` call receives `goal_armed`; the persisted Goal remains active and no Plan entry is committed. A Goal run admitted for revision 1 cannot report completion of revision 2 after a human edit, even if its tool arguments name the current revision. The Journal records a refused tool result and no unauthorized completion event.

The existing `create_goal` path still suspends for human approval, including under the session's `never` approval policy; denial leaves no durable Goal or admitted round. Model schemas expose no Goal edit, resume, or clear tool.

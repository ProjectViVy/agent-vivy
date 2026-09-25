# PG-4 Task 1: model work tool authority

The model-facing WorkControl adapter now reports an armed Goal conflict as `goal_armed` and binds `report_goal` to the exact Goal reference admitted for the calling run. The existing Service, Journal, Policy, and Eino tool path remain the owners of work, approval, and execution.

This delivery does not implement the PG-4 compound human handoff, add model edit/resume/clear operations, or change Eino dependencies. PostgreSQL acceptance remains deferred under the current user instruction.

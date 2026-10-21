# Acceptance

1. Submit a continuable child admission twice with the same operation, task, normalized tools, authority and parent owner. The prior child and first activation are returned; no second child is admitted, including after reopening storage.
2. Reuse that operation after changing the task to same-length text, selecting different tools, changing policy authority or parent owner. Admission refuses before a new Run or mailbox owner exists.
3. Complete an activation and continue under an authorized parent Run. The stable child identity, subset ceiling, direct mailbox sequence and authorizer/activation lineage remain intact; an exact activation retry returns its original Run.
4. Open a converted child binding with unavailable original input. History and pending mailbox messages remain readable, while reuse, continuation, direct activation and checkpoint recovery refuse with exact-input-unavailable/review-required semantics.
5. Retry Work with the same operation and exact normalized intended input. Return the prior result; changed intended input, scope or durable operation content conflicts. Unknown old receipt input never becomes equal to an empty request.
6. Public Work events and replay JSON contain no private request input, and child binding JSON contains no private task or authority declaration. New SQL Work payloads also exclude private request input; only the owner column retains it.
7. Pending Plan review/cancellation, Goal-to-Plan pause/drain and child-to-parent mailbox behavior retain their existing lifecycle tests. Run the integrated regression suite after owner migrations and shared runtime adapters land.

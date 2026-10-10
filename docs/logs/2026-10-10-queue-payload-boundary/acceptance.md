# Acceptance

Submit steering text close to the configured encoded journal payload ceiling. A turn whose full future follow_up restoration marker exceeds the ceiling must return the payload-limit error before acknowledgement; queue state and journal entries must remain unchanged, including after reconstruction.

A bounded steer must retain steer track in its returned DTO and durable queue event. If checkpoint resume is unavailable, its full accepted text must be admitted as a follow-up through the existing RunWithOptions path and consumed durably.

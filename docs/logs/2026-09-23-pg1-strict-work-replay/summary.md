# PG-1 Task 1: strict work replay

Round admission now requires an active Goal. Both storage backends validate the durable work stream through the domain reducer before returning a replay page, so corrupt payload versions and invalid lifecycle transitions stop work access even when the requested page starts later.

This is a partial PG-1 delivery, not acceptance of the whole PG-1 story. It reuses the existing migration 026 work-event stream, migration 027 history anchors, and WorkSeq/WorkVersion contract. It adds no new schema or storage interface. Atomic Goal run admission, review settlement, and user-facing Plan/Goal work remain outside this task.

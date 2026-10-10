# Admitted user source capture

A completed turn containing a private random user fact used to persist only the assistant acknowledgement. Capture now reads the admitted run's durable user messages, filters by run/session/role, redacts sensitive strings, and encodes `vivy.conversation-source/v1` with explicit role and completeness. A bounded assistant summary is optional and explicitly incomplete. Missing durable user source or mismatched terminal session fails closed. Existing observer receipts and Garden/Mentle authority remain the only write path.

Scope: runtime capture and real DIVA composition regression. Reflection, automatic recall and process crash acceptance remain open; this checkpoint does not mark S04 Done.

Eino capability check: this repair uses the existing MessageStore and Service.Run/Journal admission identity; Eino messages/callbacks are transient and do not supply the durable admitted-source authority. No new orchestration or provider adapter is introduced.

No release: isolated local commits only; product source pins remain unchanged pending candidate repack and complete acceptance.

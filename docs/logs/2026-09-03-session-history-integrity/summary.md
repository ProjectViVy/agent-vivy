# Session history integrity

Rewind and fork now commit their markers, copied session data, attachments, and audit events in one backend transaction. Truncation reads fail closed across RPC, model context, and trajectory projections. Chat history actions retain their editor or confirmation context until the request and resulting refresh succeed.

No stored rows or schema versions are migrated.

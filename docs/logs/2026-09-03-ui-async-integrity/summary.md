# UI async integrity

Marketplace queries now use latest-request ownership and per-skill operation state. Compaction history binds responses to the active request and keeps prior records on refresh failure. Network provider and HTTP drafts have independent dirty ownership and localized save feedback.

No visual redesign or settings wire-format change is included.

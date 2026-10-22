# Finite conversion fragments

These fragments belong only to the separately delivered offline copy converter.
They do not broaden the accepted source lineage or authorize production access.
The final runtime contains no old-key computation or fallback lookup.

## Channel association

Migration `047_channel_session_identity` adds nullable `sessions.channel_name`,
`channel_chat_id` and `channel_topic_id`, with an all-null/all-present constraint.
`sessions_channel_identity` uniquely indexes the tuple where channel_name is
not null. Bind all three columns in one UPDATE, retaining session IDs and all
references, historical payloads and titles unchanged.

Old session identity was `sess_ch_` plus the first eight SHA-256 bytes in lower
hex, over `channel + NUL + chat_id + NUL + topic_id`. The finite converter must
prove the full tuple against that old identity. Full source-owned
`channel_deliveries` rows retain `session_id/channel/chat_id/topic_id`; completed
deliveries may have been deleted. Messages and `channel.inbound` events retain
channel/chat but omit topic; titles are mutable delimiter-formatted display
text. Neither proves the missing topic. A private explicit original tuple may
be supplied and checked against the old identity. A missing, conflicting,
colliding or ambiguous association is `source_binding_unavailable`; preserve
source history and refuse publication. Never choose a default topic.

New sessions use `sess_ch_v2_` plus independently raw-URL-base64 encoded
channel/chat/topic components joined with dots. Exact session-row tuple lookup
is authoritative and returns old IDs where those bindings exist.

## Observer progress

Old key: `observer/run/` plus full lower-hex SHA-256 of
`strings.TrimSpace(provider_id) + NUL + run_id`. Snapshot rows contain only
key/value/version, so they cannot recover the provider by themselves. Match
keys offline against explicit private provider identity inventory crossed with
the stored Runs. Require one tuple per old key, reject unmatched/ambiguous
input, and reject collisions with another destination. New key:
`observer/run/v2/<rawurlbase64(trimmed-provider)>/<rawurlbase64(run_id)>`.
Copy value bytes and version exactly, delete the old key only within the same
copied-target transaction, and verify every original cursor. Final runtime
read-only admission rejects any remaining old observer namespace.

## Memory history

Keep record IDs, body bytes, revisions, scope, links and tombstones. Existing
`provenance.source_id` is the delivery identity. Old AppendHistory trimmed its
source string; future code preserves exact bytes. The known producer is
kind=`history`, provenance.source=`session_sync`, trust=`observed`,
sensitivity=`internal`, confidence_bps=10000, and correlation
request_id=`history_append-<record-id>`, turn_id=`history_append`. Its single
experience_journal evidence has ID=URI=`run:<original-run-id>`.

For that proven producer only, recover the final colon-delimited sequence from
the old source_id. It must be canonical `strconv.FormatInt` decimal fitting
int64 (zero and negatives were not rejected by the old producer). Recover the
exact Run ID from the original evidence URI; never from rendered content or
sanitized IDs. Require `strings.TrimSpace(run_id + ":" + seq)` to equal the old
source_id, then restore exact original source_id. Reject missing/conflicting
proof or duplicate exact sources. Other historical text remains untouched.

Runtime resolves exact source_id in existing history/session_sync machine-scope
records, including hidden records, before new allocation. New record IDs use
`run-history-<rawurlbase64(run_id)>.<seq>`; receipt IDs use
`vivy.memory.bml/<rawurlbase64(run_id)>/<seq>`. Old receipt strings have no durable
receipt owner to recover; preserved observer progress and record source
identity prevent duplicate ingestion rather than inventing historical receipt
authority.

All fragments stage and validate before publication, preserve copied-target
rollback, and leave the source untouched on missing proof or failure. The lead
and finite-artifact owner assemble and verify these fragments against the
actual target commit; this runtime lane makes no converter completion claim.

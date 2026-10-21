# D09 stateless history cursor

History search/read now use bounded, untrusted cursor v2 containing actual
normalized request values and source positions. Every request reauthorizes the
server-selected scope and checks the cut against current authorized storage
bounds. There is no cursor key, HMAC, fallback digest or token cache. Existing
old cuts survive a service restart and exclude later inserts; current deletion,
rewind and edit visibility still applies. Version1 cursors require a new search.

Accepted history scope stores canonical destination/source IDs directly; its
retired digest is removed from new domain values and run.started schema. Legacy
Journal JSON stays untouched and ignored extra fields grant no authority.

An oversized response is refused with narrow_scope rather than dropping scanned
records while returning an advanced cursor. The existing HistoryPicker shows
refused pages as errors, retains earlier results and selection, and offers a
search narrowing hint. It no longer presents these failures as an empty search.

This cut adds no migration and does not change runtime/orchestration ownership.
The existing Eino/native path remains; pinned API inspection is recorded in D09.
Reference exact-preview binding, shared receipt inputs, durable presentation
bytes and immutable download protocol remain the following D09 cuts. D09 Story
acceptance, owner-deferred3015 smoke and D14 final release remain open.

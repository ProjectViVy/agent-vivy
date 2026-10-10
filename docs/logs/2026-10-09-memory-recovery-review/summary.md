# Recovery review corrections

Fresh whole-branch review found three Important gaps. Canonical observations now read actual `memories.content` by RecordID and require equality with the accepted source envelope, preserving user role/run/session in separate process phases. MCP child configuration isolation is repaired in the corresponding Laputa commit. Capture now rejoins an existing durable receipt under the trusted binding/event before constructing a new payload, avoiding a source-format upgrade conflict when an old payload was accepted but the observer cursor was not persisted. The original content is not rewritten and direct changed-payload submissions still conflict.

Additional CI fixture corrections: VIVY CODE's linked-parent test now uses a temporary shared storage root instead of user home. NotifyInput's test observes the automatically woken workflow and its actual window rather than racing a second admission against it.

No release. Previously accepted assistant-only records are retained; repairing their missing source facts requires a separate governed mutation and evidence. New user-source capture is verified independently.

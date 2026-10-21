# D09 file input slice

File write/patch/multiedit/download approval uses exact bounded private bytes and
existence inside D03's existing v2 ProviderData envelope. Public generic approval
precondition fields and mutation/download SHA results are removed across domain,
repository, RPC, SDK, TUI and Web review projections. Skill review retains its
existing private revision authority; D10 still owns its separately gated closure.

The existing file version chain compares retained content directly. File reads
persist private exact state through the same storage/recorder owner; never-read
paths retain advisory behavior, while historical NULL state requires another
read. The ToolWorld Host caller uses that same recorder. Both native file and
Host writes recheck bytes/existence and authority at commit, including no-op
requests. Download rejects an already stale target before network I/O and
rechecks after streaming before rename. Failed writes preserve concurrent bytes
and remove their scratch files.

Paired038_file_approval_before_state follows integrated immutable D06 migration037.
It drops approvals.precondition_hash and file_versions.content_hash, adds nullable
private read columns, and preserves historical Journal payloads/version bytes.
Old timestamps do not reconstruct lost content. The SQL is a business fragment
for D14's finite copied-target assembly; no runtime normalizer or second runner
is introduced. Missing legacy before-state requires re-presentation.

This is the first D09 slice. Stateless history cursor v2, reference selection,
shared ContinuityStore exact request inputs, durable presentation bytes and
immutable ItemID/TransferID downloads remain to implement. D09 Story acceptance,
D10/D11 prerequisites and D14 final converter/release are not closed here.

Pinned Eino v0.9.21 adk/filesystem Backend.Write/Edit and EinoExt
agenticopenai v0.2.4 Generate/Stream were inspected as recorded in D09.md. The
existing EinoFilesystemBackend/native ToolHost path remains. The shared
std/tool-world@v1 Host changes only its internal file recorder adapter; Provider
SDK, T2 source selection, Grants and existing Host lifecycle stay authoritative.
No model/orchestration pipeline is added.

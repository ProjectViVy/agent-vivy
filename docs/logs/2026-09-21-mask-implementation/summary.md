# Mask implementation progress

Issue #43 implementation is active on `feat/issue43-mask-system` after the
architecture and Story package were accepted for execution in the active
session. MASK-1 and MASK-2 are implemented; MASK-3 now includes the runtime,
storage, checkpoint and native Eino prompt seams.

The implementation persists an immutable prompt snapshot with each first-party
admission, captures custom-definition revisions and selection CAS values,
binds the snapshot on first and resumed execution, rejects incompatible or
missing snapshots before model/tool continuation, reserves the authoritative
instruction in the context budget, and keeps runtime dependent only on the
narrow mask Resolver plus immutable prompt assets supplied by App. A missing
custom capture is classified as `mask_unavailable` so capability loss is not
reported as object absence. Sealed App compositions fail closed when the
Generation identity or atomic admission store is missing. Edit markers carry a
run ID through migration 025 so an idempotent retry cannot omit a marker and be
mistaken for the original admission.

The UI implementation slice now includes a typed `chat.header` host slot,
backend-advertised independent code mode, and the removable
`plugins/vivy-masks-ui` Module with epoch/CAS selection state, lazy definition
reads, revision-safe drafts, and EN/ZH copy. It is source-catalog registered
but not selected by the default Recipe while the legacy shell rule and complete
artifact/browser release evidence remain open.

MASK-4 browser UI, explicit selected/omitted/backend-only artifact evidence,
and the required focused `ui/AGENTS.md` correction remain pending. No push,
merge, publication, or issue comment was performed.

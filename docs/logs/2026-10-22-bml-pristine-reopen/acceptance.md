# Acceptance

After shutdown or a delivery timeout interrupts the first memory-store schema
transaction, a later app composition can read the inspector-proven pristine
file as an empty memory projection. Repeated warmup and reopen leave its bytes
unchanged. A subsequent normal append creates schema2 authority, persists one
history record, and reopens/replays that record without duplication.

Unknown nonempty SQLite data and recognized schema1 data still return their
existing explicit refusal errors, on both warmup and append, without changing
source bytes. The new branch never treats an unrecognized source as empty.

Deterministic native BML tests cover this boundary. The parent must recheck the
original integrated App restart case at the final runtime commit. Formats and
conversion mappings are unchanged, but the finite delivery's runtime pin must
be resealed and its independent reviewers must inspect the corrected candidate.

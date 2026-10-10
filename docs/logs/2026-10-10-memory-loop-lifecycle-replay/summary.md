# Memory lifecycle and capture redelivery proof

Added real-App tests for public correction/tombstone and process restart, original committed receipt stability, and accepted-source redelivery through the actual ObserverHost. Same event/content reuses the actual ingestion ID and sequence; changed content returns event_conflict. Three controlled cursor rewinds preserve raw/canonical body, hash, ID, revision and count, and restart preserves that commit.

No production behavior changed. Native create deliberately mints a fresh ID; a new human create is not receipt replay or resurrection of the tombstoned ID. Public lifecycle proof does not yet prove corrected/deleted ordinary memory in later Agent model input (S08/S09).

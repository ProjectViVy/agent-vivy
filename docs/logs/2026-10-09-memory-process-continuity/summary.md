# Memory source process continuity

The integration suite starts two distinct OS processes from the SDK-bound DIVA test binary. The first runs a real App turn with a random user fact and waits for real canonical ingestion; it exits normally. The second starts a new App against the same isolated profile and verifies source, role, ingestion/record identity, canonical count and revision. The parent checks distinct PIDs and enforces child deadlines. Fixture setup is shared with the composition smoke.

This is a controlled provider/real storage continuity proof. It is not Windows/native UI, model recall or crash-cut acceptance. The existing unsupported Restart method stays explicit until the reusable process protocol covers the broader fixture contract. No release.

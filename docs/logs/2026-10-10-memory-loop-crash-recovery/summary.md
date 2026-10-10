# Durable interrupted workflow classification

A real held-reflection subprocess crash exposed a production recovery commit collision: the pinned INOFY engine restarts transition ordinal one in generation zero, reusing its admission commit ID with different recovery content. The Core Storage adapter now qualifies only running-to-recovery_required commits with their writer epoch. Ordinary and original admission IDs/receipts remain unchanged. Unknown workflows are not replayed.

The test fixture now kills only its owned process, observes the original SQLite lease and waits actual expiry before a fresh process starts. Startup failures retain bounded diagnostics instead of opaque EOF. No lease rows are removed or seeded, no clock is replaced, and no dependency/cache source or user profile is changed.

The actual new process retains the original Run/window/native source, durably reports recovery_required, keeps watermark zero and invokes no model after manual triggers plus more than two automatic ticks. This is a pre-effect developer interruption probe, not any of the six formal ten-sample crash cuts. Full original-operation receipt recovery, half-batch failures, formal matrix and final new-source CI/artifact/conformance/review remain pending.

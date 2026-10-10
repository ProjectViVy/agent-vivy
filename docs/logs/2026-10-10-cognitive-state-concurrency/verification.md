# Verification

RED: same base revision admitted two policy writers with zero conflict; policy save erased accepted SourceHigh=7; policy save erased the actual admitted native workflow's run identity and PendingThrough=5. Raw records: chain-cognitive-concurrency-red.jsonl and chain-cognitive-admission-overwrite-red.jsonl.

GREEN: three policy/input/admission cases repeated ten times, 30 pass, zero skip/fail, exit0. Four concurrency cases with -race repeated three times, 12 pass, zero skip/fail, exit0. Final targeted runtime:85 pass/zero skip/fail/exit0. Final diagnostic actual DIVA App composition:12 pass/zero skip/fail/exit0. Final raw files are copied to the paired DIVA v0.4 checkpoint.

The 200ms scheduling gate gives a competing transaction opportunity to finish or remain serialized, then releases the captured read and asserts final durable state. It is a unit scheduling aid, not evidence of a process-crash handshake. Native source sealing, required complete just ci and whole-branch review remain pending for the final local source.

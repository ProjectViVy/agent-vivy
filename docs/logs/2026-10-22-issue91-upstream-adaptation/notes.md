# Integration decisions

Use the existing revision/receipt owners and complete admitted bytes. The
material alternative, replacing retired hashes with another opaque-content
fingerprint or a parallel replay ledger, would duplicate authority. ProgramID
and child operation IDs belong to existing workflow/Run owners; exact input,
policy, scope, implementation versions and native checkpoint contracts retain
the execution fences. Protected workflow results and checkpoint envelopes move
into the existing atomic SQL step owner, avoiding a second content-addressed
staging path. Full owner and end-to-end checks remain required.

Eino capability check: the unchanged v0.9.21 compose.WithCheckPointStore,
compose.WithCheckPointID, compose.BatchResumeWithData and ADK typed Runner /
ResumeWithParams surfaces remain the native execution/checkpoint mechanism.
INOFY's reviewed RunStore private AdmittedInput/ResumeKey/ResumeAnswers seam
fits the existing atomic storage adapter; no Vivy runner, checkpoint codec or
generic receipt service is added. Eino imports remain quarantined.

Laputa strategy identity/version uses its actual laputa/evolution/inofy
StrategyID and StrategyVersion constants. PolicyRevision is the existing
durable control-owner revision, read again at admission and after restart.
Prepared frozen authority retains its actual validated sections and text;
capture sends complete content to the upstream receipt owner. No local
content-to-identity conversion remains on those seams.

Scoped independent review found that the former cognitive task truncation
would make two changed tails compare equal under the new operation identity.
Keep the complete Prompt/InputJSON/OutputSchema and let the existing one-shot
64KiB bound refuse oversize input before admission. Regression: a changed
tail beyond5KiB must conflict under one stage operation; oversize input must
not create a child. The reviewed base64 child-ID validator is owned by D11.

The owner explicitly authorized the concrete Workflow044/045 column/index
removals and corresponding offline copied-target conversion after automatic
approval review rejected source authoring twice. The new authorization was
accepted; no production database was opened. Preserve old output blob bytes
before reference removal, keep unbindable state historical/recovery-required,
and never infer actual answers or approved inputs from a digest or public event.

The owner requested all automatic work first, followed by an isolated Web
page for manual real-model acceptance. No available credential environment
names were found in the inspected Windows/WSL processes; no values were read
or logged. The real-model verdict stays pending that manual acceptance.

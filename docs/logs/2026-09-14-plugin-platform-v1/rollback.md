# Plugin Platform v1 rollback evidence

Date: 2026-09-14

`TestSCXCandidateRollbackRestoresPriorSealedGenerationWithoutJournalMutation`
performed the release path rather than simulating a state change:

1. pack a prior full-UI Generation
   `ce9ce42386a547ed8912dbd697b0e7757743b28ccac3bde952f2df84badd2b56`;
2. pack and boot the SCX candidate
   `1dd5174c38c276b96eb1a50202cc72789c690827a501d483dfa7229faa855fb6`;
3. create real Studio ledger, evaluation, and human release records;
4. install the prior release, then the candidate release;
5. call `Studio.Rollback` and restore release `rel_700180b961a0cd1a`;
6. re-extract the installed executable's embedded Manifest and confirm the
   prior Generation ID and canonical bytes;
7. boot the restored executable on the next launch;
8. confirm the tenant Journal sentinel remains byte-identical.

The exercised selector passed in 49.93 seconds. The complementary generic
rollback test proves catalog digests and English/Chinese locale identity move
with the sealed Generation. Minimal removal is separate from rollback:
rollback atomically restores a prior whole executable and never attempts code
hot swap or plugin-specific Journal rewriting.

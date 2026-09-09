# Summary — Chat Input Toolbar Fake-Action Cleanup and Closure (UI-COMPOSER / UI-CHAT-TOOLBAR)

## What changed

The `ChatInput.tsx` top bar was aligned with the canonical project architecture requirement ("Controls without authoritative capability bits remain hidden; do not substitute demo fake controls"), completely clearing historical leftover fake controls and nonfunctional paper cuts:

1. **AutoDream button completely removed**:
   - Removed the permanently visible `GitBranch` icon button from the top bar and its click tooltip `chatInput.autodreamUnavailable`.
   - The kernel lane for this capability, MEM-1, remains DEFERRED; if a capability proposal is made in the future, it will be presented as a real feature/capability switch through the proper lifecycle rather than remaining as a useless fake control.

2. **Execution-mode dropdown consolidated**:
   - Completely removed `ask` (question mode) from the `MODES` and `ExecMode` types; clicking it only opened an unimplemented error dialog.
   - The dropdown now retains and displays only the two execution modes with real end-to-end system support:
     - **Agent mode** (`agent`, mapped to the kernel `RunModeNormal`)
     - **Plan mode** (`plan`, mapped to the kernel `RunModePlan`)
   - Simplified menu selection handling and removed the unreachable branch with no kernel semantics.

3. **i18n entry cleanup**:
   - Synchronously removed the dead keys used only by the fake actions above from `zh.ts` and `en.ts` (`askMode`, `askModeDesc`, `askUnavailable`, `autodreamTrigger`, `autodreamUnavailable`), keeping the bilingual dictionaries strictly symmetric and minimal.

4. **Automated regression and specification coverage**:
   - Updated the browser end-to-end spec `ui/e2e/thinking-gate.spec.ts` to assert that the AutoDream button no longer appears, the execution-mode dropdown contains only Agent and Plan modes, and question mode no longer appears.
   - Added `project` path normalization (`filepath.EvalSymlinks`) in `internal/codeface/launch_test.go`, eliminating false positives when comparing Windows short and canonical paths.

5. **Archival and closure**:
   - For state persistence, the earlier review decided to follow Crush semantics ("thinking and execution modes are selected per turn; there is no session-level persistence").
   - Formally marked `UI-COMPOSER` and `UI-CHAT-TOOLBAR` DONE and archived them as closed in §0.1 and §10 of `docs/TODO.md`.

## Scope / What was explicitly not done

- **Kernel Ask / read-only question semantics**: this pass was frontend-only subtraction; it removed the fake control directly and did not invent a third RunMode in the kernel without a proposal.
- **AutoDream backend capability**: MEM-1 remains DEFERRED and will be integrated through the proper lifecycle if a complete memory-system capability proposal is made in the future.
- **Session-level persistence for thinking/execution modes**: per-turn selection remains in place (Crush semantics).

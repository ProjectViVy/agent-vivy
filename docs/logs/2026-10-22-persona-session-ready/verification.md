# Verification

Base: f6b0a934ef85403339e5dffcf8e47c46208ac6e6. Independent Linux clone: /home/mastwet/work/vivy-issue91-persona-ready/agent-vivy, with its own clean sibling Laputa at the reviewed pin. Existing verification clones were preserved.

Parent-reported browser evidence on the frozen base: a cognitive status request preceded the session/messages response; the page displayed module action is not authorized. Waiting for the actual host messagesPhase ready/empty and using its Retry loaded the correct original document and allowed stale-CAS rejection/fresh save. A full page reload reproduced the initial failure. This report is causal baseline evidence, not a post-fix browser claim by this lane.

RED: pnpm --dir ui exec vitest run ../plugins/vivy-persona/ui/vivy-persona/src/view.test.tsx. Two new regressions failed while the eight original view cases passed: the status action ran before the deferred simulated session/messages binding, and session-loading error state was ignored.

GREEN: pnpm --dir ui exec vitest run ../plugins/vivy-persona/ui/vivy-persona/src. Two files, 13 tests passed. The binding regression asserts that no cognitive RPC precedes successful binding, the original document then populates, editing exposes Save, and the existing document revision is sent. The failure regression asserts a visible session-load error, host selectSession retry, no cognitive calls while retry is pending and valid document loading afterward. Existing dirty-draft, selected-kind stale-response and busy-control tests pass.

PASS: pnpm --dir ui typecheck, including its normal SDK stage-ui pretypecheck and full tsc --noEmit. Dependencies installed offline with a frozen lockfile in the independent clone. Generated Persona source derives from the authoritative plugin; no hand-edited assembly or new manifest identity was needed.

PASS: git diff --check. Focused source review confirms no backend/server authorization or concurrent-read change.

Not rerun here: parent native/full CI, browser smoke and final independent audit. The parent must rerun actual /persona cold navigation/reload, stale CAS and human save against the integrated candidate before claiming final acceptance. No heavy gates ran concurrently from this lane. The temporary transfer bundle and task output logs were removed after recording evidence.

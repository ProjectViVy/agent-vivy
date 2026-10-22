# Persona waits for the selected session binding

On a fresh /persona page load, the host publishes activeSessionId before the successful session/messages response binds the RPC peer. Persona previously issued its cognitive status action immediately, received the valid server-side authorization denial, and never retried when the session became ready. The editor remained empty and could not expose a dirty Save action.

The page now waits for the existing FaceStoreState.messagesPhase ready/empty state before loading cognitive data. The readiness transition starts the normal status/document/review lifecycle automatically. Existing request sequence, session/kind guards, dirty-draft confirmation and revision-based saves remain. A failed session load displays its existing error and retries through the host's existing selectSession method, rather than spinning indefinitely or issuing another unauthorized action.

The change adds no binding RPC, backend authorization exception, concurrency policy, state owner or runtime capability. Eino capability check: this is UI readiness orchestration over existing host state, with no new LLM/runtime capability and no Eino adapter change. Pinned Laputa remains 9acc999301d5d575da2cea902a1c6d213763f381.

Scope is the Persona view, dedicated tests and this record. Whole-candidate native CI and final browser acceptance remain in the parent integration lane. No generated file was hand-edited and no production state was accessed.

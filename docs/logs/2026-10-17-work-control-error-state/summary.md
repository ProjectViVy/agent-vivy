# Work-control error state on session switch

Date: 2026-10-17
Slug: `2026-10-17-work-control-error-state`
Pull request: [#76](https://github.com/ProjectViVy/agent-vivy/pull/76) → `main` (branch `fix/work-control-session-binding`, one commit, branched from `main`; every file it touches is byte-identical on `main` and on `feat/issue-58-unified-context`, so it does not stack on #71)

## Report

Switching sessions in the split UI showed a bare strip above the composer:

```
工作控制错误：session is not bound to this connection        [刷新工作状态]
```

The string is an internal RPC invariant, untranslated, and the offered action
was the only reason the message ever disappeared. The display was the symptom;
the defect was that the backend manufactured the failure.

## Root cause

`internal/rpc/work_control.go` gated work **reads** (`session/work/get`,
`plan/get`, `session/work/subscribe`) on the connection's single-slot session
binding. That binding is only written as a side effect of a *successful*
session read (`session/messages`, `session/todos`, `session/context`,
`session/work/get`, ... all call `peer.bindSession` after their own
authoritative `SessionStore` lookup).

`authorizeWorkSession` checked the binding **before** its own lookup, while
every sibling read binds **after** theirs. `store.selectSession` fires
`session/messages`, `session/todos` and `session/work/get` together, so the
work read deterministically loses that race: the user is switching to the
session the read names, and the read is rejected for not being bound to it yet.
Clicking 刷新工作状态 then succeeded, because the sibling reads had bound the
connection by then.

The binding gate is deliberate for **mutations** — `plan_goal_identity_test.go`
pins that a `plan/enter` for a session the connection did not select is a
conflict — and that behaviour is unchanged.

## Change

1. `internal/rpc/work_control.go` splits the resolver:
   - `resolveWorkSession` — authoritative `SessionStore` lookup, no binding
     gate; used by the three work reads. Their success is what binds the
     connection, exactly like the sibling session reads.
   - `authorizeWorkSession` — `resolveWorkSession` plus the connection-binding
     conflict, used by work mutations only.
   `internal/rpc/work_subscription.go` uses the read resolver.

2. `ui/src/components/chat/WorkControlBar.tsx` renders one failure surface for
   both error cases (no work state, and a failed action while work state is
   shown) through the shared `RecoverableFailure` (`compact`), with the
   existing `workControl.refresh` label as the retry action. The failure keeps
   the shared classifier's explanation (so a lost control plane reads as
   "控制面没有响应…") but takes the work-control title, and does not inherit the
   Run hint "这不是后端连接问题…" that does not apply to work state.
   `ui/src/lib/failure.ts` widens `ClassifiedFailure.titleKey` by
   `errors.workControlTitle`; `errors.workControlTitle` is added to both
   locales and the now-unused `workControl.error` template is deleted.
   `scripts/i18n-cross-face-contract.json` swaps `workControl.error` for
   `errors.workControlTitle` in the Web face-specific key list, which is what
   the cross-face gate asserts.

3. When the conversation itself failed to load, the message area already owns
   that one explanation and retry (`ChatView`), so the work bar renders
   nothing instead of repeating the same control-plane failure twice.

## Not done

- No change to the mutation binding gate, the ActionHost identity path, or
  `bindSession`/`bindRun` provenance rules.
- No generic backend-message-to-user-copy mapping was introduced; only the
  existing shared failure classifier is reused.
- The `just ci` UI build step could not run to completion in this session's
  shell; see `verification.md`. It is an environment failure reproduced
  without this change.

# Vivy UI

The browser-based Vivy workbench is a zero-runtime-dependency Vite + TypeScript
shell served by the Go binary. It talks only to the Vivy JSON-RPC control plane
and its journal-backed run event stream.

The surface is organized around four user tasks:

- sessions: identify, select, rename, create, and delete conversations;
- live activity: discover active runs across sessions and attach to their
  persisted state after navigation or refresh;
- conversation: send one turn at a time, see streaming output, and preserve the
  draft across preflight or network failures;
- run inspector: inspect durable events, structured failures, scoped
  approval/question tasks, and the durable parent/child run tree without
  blocking unrelated session navigation. Child runs can be opened and cancelled
  through the same backend-authoritative lifecycle controls.
- review center: review pending approvals and user questions across sessions,
  inspect redacted arguments/diffs and risk context, and make decisions from
  the same renderer used by the inline run inspector.

The UI follows the system language and theme on first launch. The language
toggle supports English and Simplified Chinese; the theme control cycles system,
light, and dark modes. Preferences are non-sensitive local UI settings only.

Build and verify from this directory:

```powershell
npx tsc --noEmit
npm run build
npx playwright test
```

The Playwright smoke starts the real Go process with the deterministic provider;
it does not replace the production user path with mock domain records.

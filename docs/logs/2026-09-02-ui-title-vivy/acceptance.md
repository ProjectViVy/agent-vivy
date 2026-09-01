# Acceptance

How a human can tell this worked:

1. Open `http://127.0.0.1:3015` — the browser tab reads **VIVY** before and after the app finishes loading (static `<title>` and the runtime i18n override agree).
2. Switch UI language (en/zh) — the tab title stays VIVY in both.
3. `ui/e2e/app-title.spec.ts` keeps this pinned in CI.

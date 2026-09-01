# Verification

| Step | Command | Result |
|---|---|---|
| Product gate | `just ci` (root) | CI-EXIT:0 (tail-checked) |
| Real-path smoke | `just ui-e2e` (3015 dev server) | app-title.spec: browser tab title = VIVY; suite green |
| Static value | `ui/index.html` `<title>` | `VIVY` |
| i18n values | `app.documentTitle` en/zh | both `VIVY` |

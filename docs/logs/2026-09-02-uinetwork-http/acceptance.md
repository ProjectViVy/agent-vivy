# Acceptance: http_request tool surface

## Human view (browser, http://127.0.0.1:3015)

1. Settings → Network tools section: an "http_request tool surface" block appears below
   the card, showing the configured default allowlist (localhost/127.0.0.1/::1) and configured
   timeout (10 seconds), with no "Settings override active" badge.
2. Enter `api.example.dev`, `*.corp.dev` in the allowlist textarea, change the timeout to 45,
   and click Save → a "Saved; takes effect on next startup" confirmation and a "Settings
   override active" badge appear.
3. Refresh the page: the saved allowlist and timeout are echoed, and the badge remains
   (`settings.yaml` http section persisted).
4. Set the timeout to 999 → the Save button is disabled (the 1–120 clamp boundary is
   intercepted first in the UI).
5. After saving the default values again, the effective allowlist/timeout matches the config
   (overlay and config have the same values).

## Backend effect

- No restart is needed after writing settings.yaml: the next `http_request` call uses the
  new allowlist/timeout (SetConfig live-application seam).
- Config defaults (with no overlay) behave exactly as in the historical version.

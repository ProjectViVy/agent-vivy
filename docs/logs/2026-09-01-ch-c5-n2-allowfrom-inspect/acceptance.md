# Acceptance

- Open the app at `http://127.0.0.1:3015`, Settings → Channels.
- Edit only the allowed-senders list of a channel and save: the card /
  list entry / editor header now show the "Saved; applies after restart" badge
  (previously a pure allow_from edit showed no pending-restart signal at all).
  After a process restart the badge
  clears.
- Stop the backend (or point the UI at a down control plane) and refresh
  the channels page: the main area shows an error panel with the failure
  reason and a refresh button — not the "This generation has no ears" empty
  state.

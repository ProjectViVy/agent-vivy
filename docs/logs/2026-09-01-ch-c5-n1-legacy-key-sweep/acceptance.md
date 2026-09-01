# Acceptance

- A browser that once ran the old frontend (which stored
  `vivy.ui.channels`) shows the key absent from DevTools → Application →
  Local Storage after opening the new UI's Channels settings once.
- Fresh browsers are unaffected (no reads, no writes of channel data
  client-side).

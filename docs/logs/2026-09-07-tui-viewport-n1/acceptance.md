# Acceptance: how a person can tell it works

1. In a long session, have a tool call produce multiple lines of output (e.g. run a loop with `bash` and print).
2. Before the result returns, use PgUp / the mouse wheel to stop midway through history.
3. When the result fills the card: **the viewport's stopped position does not drift** (content above the anchor grows, and the on-screen content moves down by the same number of lines rather than the screen “jumping” away).
4. Scroll back and forth through a long session: no stutter (each frame assembles only the visible window and no longer re-renders the entire history).
5. Switch to a history session with many messages: it briefly shows “Loading session history…”, **without** first flashing the empty-conversation welcome page “Journey to Find Your True Heart”; the history then appears.

## Regression anchors

- After stopping with PgUp, `End` returns to the bottom and the scroll hint hides (existing `chrome_test` assertion).
- Within 2 lines of the bottom, the “return to bottom” hint is not shown (existing `chrome_test` assertion; relies on the fingerprint latch not interfering with direct offset writes).

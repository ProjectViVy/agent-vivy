# VIVY CODE pointer-region wheel routing

## Changed

- Routed mouse-wheel input to the pane under the pointer: the Crush-style right rail now scrolls on hover without a prerequisite click.
- Kept explicit sidebar keyboard focus independent from hover. Moving the pointer back over chat routes the wheel to chat without clearing sidebar keyboard focus.
- Kept gates, dialogs, command overlays, compact layout, and non-scrollable panes isolated from background wheel input.
- Updated the open viewport track to retain only stable-anchor, history-loading epoch, and long-history caching work.

## Explicitly not done

- No stable message anchor, render cache, or asynchronous history epoch was added in this focused interaction slice.
- No Studio source or tenant Journal was read or changed.

This is a focused source delivery, not a release; no release record is included.

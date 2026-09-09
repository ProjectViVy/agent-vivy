# Overlay and Main-Page Isolation

## Delivered

- Command palette, shortcuts, session, model, and other overlays no longer stack over chat text. When opened, they first clear the main screen and then center an opaque dialog.
- Dialogs have a solid background; overlays are clipped by cell to prevent wide CJK characters from cutting the border into the body.

## Boundaries

- While an overlay is open, the main chat/sidebar is temporarily invisible and returns after it is closed.
- No semitransparent mask was implemented (the terminal cannot do true semitransparency).

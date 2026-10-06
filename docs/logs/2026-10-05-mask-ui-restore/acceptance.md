# Acceptance

1. Open the split UI at `http://127.0.0.1:3015`, create a conversation, and
   open VIVY → Masks. The library shows role cards with identities,
   descriptions, and an apply action; details sit beside it on desktop.
2. Apply Writer from its card. The current badge and compact toolbar choice
   both become Writer. Select Researcher from the toolbar; the library's
   current choice updates immediately. Reload and reopen the conversation:
   its committed mask remains selected.
3. Inspect Programmer. Its fields are read-only and it offers duplication.
   Duplicate it: the new custom draft retains the instructions and becomes
   editable. Save and manage custom definitions through the detail editor.
4. Change a custom definition from two browser contexts. The stale save
   displays a conflict and retains the draft. Attempt deletion while a
   session references that definition: the UI reports the refusal and keeps
   the card.
5. Toggle code mode on and off. The session mask stays unchanged. During an
   active run, the mask menu explains that a new choice applies to the next
   run.
6. At a 390 × 844 viewport, the mask menu remains inside the application
   toolbar. The message composer is fully visible and the document has no
   horizontal overflow.

Automated race checks additionally verify that a late response from session
A cannot overwrite B, a pending write cannot be superseded by a pre-commit
refresh, a conflict reread supplies the revision used by retry, and an
unavailable persisted mask is never displayed as an unmasked session.

Browser captures: [desktop library](library-desktop.png) and
[mobile composer](chat-mobile.png).

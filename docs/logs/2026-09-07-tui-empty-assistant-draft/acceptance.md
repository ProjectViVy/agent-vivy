# Acceptance — how a human can tell it worked

1. Send any message in `vivy-code`.
2. Immediately after sending, the chat body stays clean: no empty purple
   assistant line with a lone `▌` cursor appears before the model starts
   producing text.
3. The first thing that appears for the reply is actual content (reasoning
   or answer text) streaming in with the cursor; the busy state is shown by
   the chrome status line as before.

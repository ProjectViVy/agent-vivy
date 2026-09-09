# Acceptance — Recoverable frontend errors

At `http://127.0.0.1:3015` (split Vite, not the `:8787` embedded UI), a user can confirm:

1. **Control plane is down**
   Stop the backend and refresh the page: the full-screen message is "Unable to connect
   to Vivy", explaining that `:8787` must be started first; the primary button is
   "Retry". Clicking Retry performs a new handshake without a full-page refresh. After
   the backend is running, click again and the session opens.

2. **Missing API Key**
   With the backend running but no key configured, send a message: the conversation area
   shows "Model key not configured" and an "Open model settings" link to
   `/settings?tab=model`. The input draft remains.

3. **Run failure reason is visible**
   When a conversation ends with `run.failed`, the chat area shows the failure
   explanation from the payload instead of going blank or changing only the run state.
   After refreshing and reopening the session, the historical failure reason remains.

4. **Send failure preserves the draft**
   If preflight or `turn/start` fails, the input text remains and can be edited and sent
   again.

# VC-0 acceptance (how a person can confirm it works)

1. **The mask determines face**: open http://127.0.0.1:3015 (split mode), switch the input-area mask
   to "Programmer," and send a message → this run uses code face: the Journal's
   `run.started` payload has `face:"code"`; switch back to the Default/Researcher/Writer masks →
   `face:"web"`.
2. **The mask card visibly upgrades**: the Programmer mask's capability list gains "Runs with the code face."
3. **Preflight echo**: in browser DevTools → Network → the `preflight/run` request body contains
   the `face` field, and the response face matches the selected mask (Programmer = code; others = web).
4. **Invalid face is rejected**: send `face:"shell"` directly to `preflight/run` → RPC error
   -32602 `runtime: invalid face`; the UI does not start a run.
5. **Resume preserves face**: create a tool-approval suspension (an approval-pending run); after approval
   resumes, the face in subsequent event payloads matches the face before suspension.
6. **Code-face prompt framing**: under the Programmer mask, ask the model to cite code locations; responses should favor
   `path:line`. A `composeRunPreamble` unit test asserts that the code-face preamble contains
   "Code mode is active" and the web-face preamble does not.

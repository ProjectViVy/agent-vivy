# Human acceptance and current limitations

Overall acceptance is **FAIL** until the repository gate and the open Toolkit
findings are resolved. Working core paths can be checked independently in an
isolated user home.

1. Start the split pair with an explicit disposable `VIVY_USER_HOME` and an
   isolated config, then open `http://127.0.0.1:3015`.
2. Dismiss the first-use wizard, enter Persona from the VIVY navigation group,
   and initialize all five required documents. A fresh uninitialized Persona
   correctly prevents ordinary conversation.
3. Send a message through the configured loopback model. Confirm its reply is
   visible, survives reload, and is copied into a forked session.
4. Select `/plan` from the composer command menu and execute it. Confirm Plan
   mode appears and Leave Plan returns the session to normal.
5. Visit the Toolbox and VIVY destinations plus Dashboard and Settings. At
   390px, open the navigation drawer and confirm the document does not overflow.
6. In an authenticated selected session, request a synthetic `write_file` call,
   approve the pending review, and confirm the run completes with the file in
   the disposable workspace. Cancel a separate hanging run and confirm its
   terminal state is `cancelled`.
7. Make the loopback provider return 503. Confirm failure is visible and the
   user's submitted text remains in history. Restart after the documented
   30-second organism lease expires if the process was force-terminated;
   confirm the earlier completed transcript remains available.
8. On a fresh instance, preview and install a native Toolkit disabled; enable,
   authorize Hook/MCP independently, save environment references, update,
   reauthorize the changed declaration, disable, and uninstall. This lifecycle
   passed with a diagnostic status locator adjustment. The original test
   locator and the post-run authorization path remain open findings.
9. Confirm the embedded UI loads and `vivy-code --help`, `--version`, and a
   journaled `--no-tools -p` call complete against the loopback model.

Browsing a page is a smoke check, not acceptance of all actions exposed there.
No claim is made about live provider response quality, live channel delivery,
Studio behavior, or sealed release conformance.

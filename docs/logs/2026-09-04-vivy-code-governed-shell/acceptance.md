# Acceptance

1. Start `vivy-code`, open a session with a writable permission preset, and
   enter `!echo hello`. The command runs through `shell/start`, renders the
   ordinary tool lifecycle/output, and emits no model request.
2. Under the Smart preset, enter a mutating workspace command such as
   `!echo changed > marker.txt`. Vivy opens the normal approval interaction;
   approving runs it only after the decision is durable, while denying leaves
   no marker and closes the run once.
3. Enter a forbidden command such as `!curl https://example.com`,
   `!cat ../secret`, `!echo x > /tmp/x`, or `!sleep 30 &`. It is rejected
   before a process starts and no raw command is echoed into durable history.
4. Cancel a running foreground command. Its process is terminated, the run is
   cancelled, and no background job id is created.
5. Restart while a shell approval is pending, then decide it. The pending run
   resumes from its opaque state reference. Restarting after execution began
   never replays the process.
6. Inspect run events, approval rows, message history, and logs. They contain
   bounded redacted command metadata and sanitized output, not the raw script,
   host cwd, or secret-like output.
7. Connect a TUI to a generation without `shell.start`. The help overlay does
   not advertise `!<script>`, and a bang input is rejected locally without a
   process, model turn, or alternate RPC route.

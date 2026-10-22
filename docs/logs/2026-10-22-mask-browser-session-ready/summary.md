# Mask browser session readiness

The browser helper now waits for the actual selected session ID to change and
for its host-owned message binding to finish before navigating to Masks.
Previously the already-visible composer allowed navigation while session
creation was still pending, and the completion callback returned to chat.
This changes test preparation only; product behavior and authorization are unchanged.

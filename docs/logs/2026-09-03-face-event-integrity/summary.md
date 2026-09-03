# Face event integrity

Headless now registers its event listener before subscription replay. The TUI uses a lossless ordered notice queue, establishes run ownership before subscribing, and keeps approval/question gates retryable until their RPC succeeds.

This delivery does not change the web UI or session history storage.

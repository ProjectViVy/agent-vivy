# Acceptance — VC-3 slice 5 (read_file images)

How a human can tell this works:

1. Run a session with a vision-capable model and a workspace containing a
   screenshot (png/jpeg/gif/webp, under the read cap).
2. Ask the model "read shot.png and describe it". The model answers from
   the image content — before this slice it got only
   `{"binary":true,...}` and could not see anything.
3. Ask about a text file as before — behavior is unchanged (numbered
   content, no `parts` field).
4. A screenshot larger than the read cap (default 1MB) returns a loud
   error naming both sizes instead of a silently truncated image.

No UI change in this slice; the attachment surfaces through the model's
own reply, not through a UI panel.

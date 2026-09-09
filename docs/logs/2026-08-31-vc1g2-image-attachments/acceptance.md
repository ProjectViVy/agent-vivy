# VC-1g-2 acceptance path (manual)

Prerequisite: `just dev` (or `just run` + `cd ui; pnpm dev`), with a browser open at
`http://127.0.0.1:3015`; configure a vision-capable provider/model in Settings and enter its key.

1. **Select files**: click the paperclip in the input toolbar → the file picker accepts only
   png/jpeg/gif/webp → select 1–3 images → thumbnails appear above the input box.
2. **Remove**: click the × in a thumbnail's upper-right corner → the image disappears.
3. **Paste an image**: take a screenshot and press Ctrl+V in the text box → a thumbnail appears (text pasting is unaffected;
   copying and pasting text still inserts it into the input box normally).
4. **Gate notices**: select an image >5MB → inline notice "Image exceeds the 5 MB limit";
   select a non-allowlisted type (for example, a .png extension whose contents are really a gif, or a .txt file) →
   "Unsupported image type". Add a 5th image → "Maximum 4 images per message".
5. **Send**: enter text and send → the user bubble shows thumbnail + text;
   the model replies using the image content (vision model) → multimodal input is working.
6. **Persistence**: refresh the page and reopen the session → historical user messages still have thumbnails
   (server data URL, not a local optimistic row).
7. **Queue with an image**: run a long task, then send another image-bearing message while it is running → it enters the queue;
   after the current run completes it is dispatched automatically, and the message/image send normally (compare VC-1g-1 behavior).
8. **Compaction safety**: trigger compaction in the session → the summary is generated normally
   (images in the transcript are `[image attachment: name]` placeholders with no binary data).

## Alternative acceptance without a key

Steps 1–4 and 7 do not depend on a model (7 requires an active run; a local echo-like tool can keep it busy,
or the queue pill behavior can be observed). Step 5 requires a vision provider; step 6 requires one
successful image-bearing run persisted to storage.

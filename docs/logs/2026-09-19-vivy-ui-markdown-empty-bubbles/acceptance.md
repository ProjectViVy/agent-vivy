# Acceptance — Vivy UI markdown + empty bubbles

Open the dev UI at `http://127.0.0.1:3015` (or the console's own frontend
button) and open a session that used tools.

1. **No stray bubbles.** Between a user/assistant message and its tool result
   cards there are no small empty bubbles. Before: every tool call left an
   empty bubble plus a timestamp/copy/regenerate/rewind/fork action bar.
   After: the tool result card is the only thing a tool call adds to the
   transcript; the assistant message that actually carries text keeps its
   action bar as before.

2. **Markdown reads as markdown.**
   - A pipe table in a reply renders as a bordered table with header cells
     instead of a paragraph of `| 类别 | 工具 | --- |` text.
   - Paragraphs are separated, list items show markers, `**bold**` is bold and
     `` `code` `` is a monospace chip.
   - Streaming still shows `…` while the first token is pending, and replies
     keep rendering incrementally.

3. **Nothing else moved.** User bubbles stay white-on-primary (the typography
   plugin's grey body colour is overridden for that bubble), attachments, the
   thinking (`details`) block, regenerate/rewind/fork confirmations, diffs in
   tool results and the todo strip behave as before.

Not covered here: tool-call detail (tool name/arguments on the result card) is
still to be specified separately.

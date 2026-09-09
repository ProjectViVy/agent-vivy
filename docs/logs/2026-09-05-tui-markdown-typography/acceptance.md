# Acceptance

1. Start `vivy-code` or `vivy tui` and have the assistant reply with Markdown containing headings, lists, quotes, inline code, and fenced code blocks.
2. The main chat should no longer expose raw source such as `# Title` / `**bold**` / `- item` / `> quote`; it should show an H1 color block, a `##` subsection, a `•` list, a `│` quote, and highlighted code blocks.
3. Markdown sent by the user is formatted the same way; identity is still indicated by the blue left-side `┃`.
4. Thinking/reasoning blocks retain the `┊` gutter and their structure, but use more subdued colors.
5. When the window narrows, line width does not exceed the viewport, and the streaming cursor still follows the end of the current bubble.
6. Tool card appearance and `tui.debug` ellipsis behavior remain unchanged.

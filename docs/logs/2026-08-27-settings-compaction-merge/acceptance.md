# Acceptance perspective (2026-08-27, merge Compaction into General)

## How a user can tell it works

1. Open the app and enter 「Settings」: the top tab list no longer shows the 「Compaction」
   tab (formerly between 「Network」 and 「Self-evolution」, with a gray 「Preview」 badge).
2. The default section is 「General」: a 「Context compaction」 card appears below the 「Chat
   display」 card, containing the history-message usage progress bar, three inputs
   `max tokens / compaction threshold (%) / retain recent messages`, and the two buttons
   「Run compaction preview」 and 「Restore preview defaults」.
3. Change any compaction setting (for example, change the threshold to 50); the feedback
   row updates immediately (for example, 「Compaction threshold preview updated.」); click
   「Run compaction preview」 and history-message usage becomes 「retain recent messages ×
   240」, with feedback 「Simulated one context compaction preview.」.
4. Click 「Restore preview defaults」: the three inputs return to 8192 / 80 / 12, the progress
   bar returns to its default state, and the feedback row shows 「Compaction configuration
   restored to preview defaults.」.
5. The deep link `http://127.0.0.1:3015/settings?tab=compaction` no longer errors or selects
   any Compaction tab; it directly displays the 「General」 section content.
6. The remaining preview sections (Channels / Network / Self-evolution / Sandbox) look and
   behave the same.

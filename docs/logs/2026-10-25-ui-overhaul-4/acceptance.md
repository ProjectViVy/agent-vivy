# Block 4 — acceptance (what a human should see)

Start the split pair (`just dev` or the isolated pair from `verification.md`)
and open the chat at 1280 and at 390.

1. **One column.** The composer's left and right edges line up with the
   transcript column at every width; at 1280 both are the 52rem reading
   column, at 390 both are full-bleed minus the same page gutter.
2. **Idle composer.** One row of quiet controls on top (history, attach,
   permission preset, approval trigger), the message field, then the footer
   with the context ring, workspace chip and the round send button. Nothing
   scrolls sideways; at 390 the controls wrap onto the space available
   instead of hiding behind a horizontal scrollbar.
3. **Typing.** The field grows line by line as you type. Paste six lines:
   the box is six lines tall. Paste twenty-four: the box stops at 40% of the
   viewport height and scrolls inside itself; the toolbar and footer stay
   put. Enter sends, Shift+Enter breaks a line, and while a run is active
   Enter steers, Shift+Alt+Enter queues a follow-up, Cmd/Ctrl+Enter sends now.
4. **Queue.** Steer and follow-up entries appear as pills above the field
   with their lane badge (`steer` / `queued`), two lines of text and a tooltip
   for the rest. Each pill has its own remove ×; `Clear queue` empties the
   row. Nothing truncates to an ellipsis mid-word.
5. **Approval.** When a tool needs approval the shield badge shows the
   pending count fully inside its button and the card pops above the
   composer: title, the command or diff wrapped to the card width, the risk
   line, Approve / Deny / Approval center. At 390 the whole card fits the
   viewport with no clipped glyph; denying closes it and the badge count
   drops.
6. **History.** The history button opens the picker as a page-level dialog
   owned by the chat view; attaching a reference shows chips under the
   toolbar, removable one by one.
7. **Switches.** The thinking menu lists the nine modes with a check on the
   active one; the permission menu shows cautious/smart/trusted with their
   one-line meanings, and choosing Trusted asks for confirmation first.
   While a run is active the permission switch says the change applies to
   the next turn.

Console stays free of errors and raw i18n keys in all of the above.

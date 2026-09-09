# Acceptance

Date: 2026-08-27

Product/user perspective: open http://127.0.0.1:3015 (split Vite + real control plane),
enter the chat page, and inspect the toolbar above the input.

## Acceptance checklist

1. **Layout and content**: from left to right, the toolbar is mode selection (Agent /
   Plan / Ask), Attachments, thinking toggle, AutoDream, companion, permission mode
   (Cautious / Smart / Trusted), divider, History, and Approvals; 「Draw」 and the separate
   「Smart」 button no longer appear.
2. **Mode dropdown**: clicking 「Agent mode」 opens three items upward (icon + title +
   description), with the current item checked and highlighted; selecting 「Plan mode」
   changes the trigger to 「⚙ Plan mode」, and selecting 「Ask mode」 changes it to
   「🧠 Ask mode」; clicking again toggles/closes it.
3. **Thinking dropdown**: clicking the lightbulb button opens Auto / On / Off; 「On」 uses a
   solid lightbulb, and the selected item is checked and highlighted.
4. **Permission dropdown**: clicking 「Smart」 opens Cautious / Smart / Trusted upward; the
   selected trigger label updates (for example, 「🛡 Cautious」 / 「✦ Trusted」).
5. **Stub feedback**: clicking Attachments / AutoDream (branch icon) / companion (cat icon)
   shows the corresponding 「Not connected yet」 notice bar at the bottom of the input,
   disappearing after about 1.8 seconds.
6. **History**: clicking the clock icon opens the 「Sessions」 drawer on the right
   (including search / create / select / rename / delete), the same drawer opened by the
   message icon in the upper right.
7. **Approvals**: clicking the shield icon opens the Approvals Sheet; when approvals are
   pending, a red-background, white-text numeric badge (such as 「2」) appears at the
   button's upper right and is hidden when none are pending.
8. **Bilingual**: after switching the setting to English, all toolbar copy (including mode /
   thinking / permission menu descriptions) changes to English; switching back to Chinese
   leaves no stale copy.

## Failure criteria

- The appearance of a 「Draw」 or old 「Smart」 button, sending remaining unchanged after
  a mode switch, a dropdown failing to open/close, the History button still showing a
  notice bar, or a non-numeric Approvals badge—any one of these means failure.

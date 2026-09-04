# Acceptance

1. Start `vivy-code.exe` in a project and type `@REA`. After the short debounce,
   a **Project files** popup lists matching project-relative text files such as
   `README.md` without starting a model turn.
2. Use Up/Down to wrap through candidates. Press Enter or Tab: only the active
   trailing `@` token is replaced, and a trailing space is added for continued
   prompting. Press Escape instead to close the popup without changing the
   draft.
3. Select a file whose name contains a space. The editor inserts a quoted form
   such as `@"docs/design notes.md"`; submitting a surrounding prompt sends the
   exact original path through the existing server-owned resolve/revalidate
   path.
4. Type quickly so multiple debounce generations are created, or switch the
   active session while a request is pending. Older results never overwrite the
   current popup. A permission/question gate closes the popup and remains the
   top interaction.
5. `@@literal` and email-like prose remain literal and do not open completion.
   A malicious absolute/traversal/control-sequence candidate is not rendered or
   selectable.

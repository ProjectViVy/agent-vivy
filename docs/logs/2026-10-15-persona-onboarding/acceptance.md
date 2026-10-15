# Acceptance (human view)

## The first minutes

1. Start the split pair at a **fresh** home:
   `VIVY_USER_HOME=.workspace/fresh just dev`, then open
   `http://127.0.0.1:3015`.
2. Finish or skip the host's own welcome wizard.
3. The persona panel then appears by itself, as a dialog: it says what this
   companion is meant to be before it asks for anything, then walks you through
   面容 → 它是谁 → 关系 → 关于你 → 世界 → 边界 with the step rail on top and
   `上一步 / 下一步` on the bottom.

You can tell it worked when:

- The panel opens on the philosophy, not on a form.
- On 面容 you can pick one of the emoji faces **or** choose a photo from your
  machine; the face you pick is in the top-left corner of the app afterwards,
  and it opens again from that corner at any time.
- Every document page shows one question and an example; going back never
  loses what you already wrote.
- Pressing the last action writes everything at once: you land on
  「它记住了。」 and can start talking.
- Nothing nagged you twice: after you finish (or after `Not now`), reopening
  the app does not show the panel again — but `/persona` still offers the same
  guided flow if you want it.

## What it must refuse to do

- With an already-initialized persona, reloading the app never shows the panel.
- With no session, or when the authority cannot answer, the panel stays absent
  instead of showing a broken form.
- If the authority rejects one document (for example, identity is over its
  character cap), you keep **everything** you wrote, you see the authority's own
  sentence, and you land on the document that needs fixing.
- The face is decoration: it never appears in a prompt, a document, or the
  Journal, and it is never uploaded anywhere. It is stored on this device only.

## Where to look

- `/persona` (sidebar → Persona) shows the eight documents, the pending-review
  tab and the source view; with an uninitialized persona it shows the same
  guided flow instead of the editor.
- The disposable home proves the write was real:
  `garden/persona/persona/IDENTITY.MD` (and its siblings) contain exactly the
  text you typed, `USER.MD` is wrapped as `## Preferences`, and
  `garden/persona/persona/history/<FILE>/1.md` keeps the first revision.

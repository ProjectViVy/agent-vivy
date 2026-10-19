# Human acceptance

With the normal Go/just/PowerShell toolchain and Recipe assembly available:

1. Start the split pair with `just dev` and open
   `http://127.0.0.1:3015`.
2. Inspect the chat toolbar at desktop and mobile widths. There is no
   Code / Life preview switch. Model selection and the Recipe-selected
   chat-header module still occupy their existing toolbar.
3. Open an ordinary session and confirm its current persona/mask remains
   selected. Selecting a mask still works independently of chat controls.
4. Check that Work, context, permission, and Plan/Act controls retain their
   existing behavior. This change does not assign session policy or modify
   the internal LIFE/Coding distinction.

The updated `ui/e2e/masks.spec.ts` acceptance case covers selector absence
and the selected Programmer mask when its packed split-pair prerequisites
are available. This environment could not execute the browser acceptance.

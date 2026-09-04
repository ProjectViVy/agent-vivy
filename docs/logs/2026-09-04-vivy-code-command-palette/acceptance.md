# Acceptance

1. Launch `vivy-code` in a fullscreen-capable terminal. The footer advertises
   `^p commands`.
2. With an empty editor, press `/` or `Ctrl+P`. A centered Commands palette
   appears without sending any model input.
3. Type `attach`; `/image <relative-path>` is the selected first result because
   `attach` is its registered alias. Other commands may remain when their usage
   or description is also a fuzzy subsequence match. Use arrows or
   `Ctrl+P`/`Ctrl+N` to move.
4. Press Enter. The palette closes and `/image` is inserted into the editor;
   it is not executed until its required path is supplied and Enter is pressed
   again.
5. Open the palette and type an unmatched query. It shows `no matching
   commands`; Enter is a safe no-op and Esc returns to the unchanged draft.
6. Type `/` twice, continue with `hello`, and press Enter. The ordinary prompt
   `/hello` is sent through the existing literal-slash path.
7. While a gate, sessions picker, confirmation, or result overlay is active,
   palette keys do not hide or bypass that higher-priority surface.

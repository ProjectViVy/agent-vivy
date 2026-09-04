# Acceptance

1. Start VIVY CODE with a writable settings home and press `Ctrl+L`; confirm the dialog says `Switch global model` and lists the current route first.
2. Confirm models from the shipped OpenAI/Anthropic bundles and models from saved custom provider entries appear, while provider Base URLs and API keys never appear.
3. Type a provider/model fragment, navigate with arrows, and press Enter (or Ctrl+Y). Confirm the modal remains while applying, then closes only after server success and the right rail reflects the selected provider/model.
4. Enter `/model <fragment>` and confirm it opens the same filtered picker. `/help` must list `/model [filter]`, and the footer must advertise `^l global model` only when the initialized capability exists.
5. While a run, gate, local queue, or selection request is active, confirm a second model transition cannot start. A server conflict must leave the picker open with the previous current marker and an actionable error.
6. In a frozen/read-only deployment, confirm the catalog remains browsable but Enter cannot mutate it.
7. Save a custom gateway whose model name overlaps another route. Confirm the gateway and direct bundle entries remain distinct by hidden endpoint identity and choosing the direct entry never inherits the custom endpoint/key.

# Acceptance

1. Open VIVY CODE at 120 columns by 30 rows or larger and complete at least one model request.
2. Confirm the right rail shows `Session Usage` with total, input, output, request count, and estimated reference cost. Reasoning and cached rows appear when their authoritative values are non-zero.
3. Confirm a fully priced free request shows `$0.0000`, while any unpriced or partially priced session shows `unknown` rather than zero.
4. With known model metadata, confirm context shows `~<percent>%`, estimated used tokens, the model window, and a warning only above 80%.
5. With unknown custom-model metadata, confirm the right rail says `limit unknown` and does not show the runtime's 128k fallback or a percentage.
6. Repeat with the built-in and packed TUI faces; the right-rail fields and semantics must match.

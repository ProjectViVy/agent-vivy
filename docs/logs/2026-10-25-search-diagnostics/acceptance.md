# Acceptance (product/user view)

A human can tell this worked when, after a failed `network_search` call:

1. The chat/conversation trace shows the tool result as a recoverable
   failure (the run continues; the model sees the text and can react), and
   the text names the provider that was actually used — e.g.
   `network search: provider bing failed: ...` — not a bare
   `search provider returned HTTP 403`.
2. When the failure is a missing configuration, the text tells the operator
   exactly which environment variable to set: `BING_SEARCH_API_KEY` for
   bing, `GOOGLE_SEARCH_API_KEY` and `GOOGLE_SEARCH_CX` for google,
   `SEARXNG_SEARCH_URL` for searxng with an empty endpoint.
3. When the failure is an upstream HTTP error, the text reads
   `search provider <name> returned HTTP <code>`.
4. The `network_search` tool description shown to the model no longer
   promises a fallback between providers; it states that one provider is
   selected and used, and that a failure names that provider.
5. Nothing else changed for the user: the tool is still available, provider
   selection order is identical, keyless duckduckgo/wikipedia still work
   with no configuration, and successful searches look exactly as before.

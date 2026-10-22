# Acceptance — custom provider runtime

With the updated backend:

1. In Settings → Model, register an OpenAI-compatible provider with a display name, `base_url`, raw model id, and API key, then select it.
2. Send `Hello` from a Tiny Face session. The run should complete, the gateway should receive `POST /v1/chat/completions`, and the request should carry the selected raw model id and configured bearer key.
3. Clear the custom provider key and retry. Vivy should report a missing key for the selected provider before sending a request.
4. The custom registry contains no model capability metadata, so Vivy must not infer thinking support or send provider-specific thinking fields.

The source and deterministic HTTP regression checks pass. A human browser acceptance against the rebuilt isolated backend is still required before D14 can be closed.

# Acceptance

For each provider request, a completed HTTP response record has the same request index, status, headers and full body accepted by the handler. A response above 4 KiB retains its Unicode canary and exports byte-for-byte. An interrupted provider request remains visible as an unmatched request; no response is fabricated. The model-count file states that client consumption was not asserted.

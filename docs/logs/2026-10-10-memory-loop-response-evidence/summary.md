# Actual HTTP response evidence

The development evidence recorder now preserves every actual provider request alongside the HTTP handler status, response headers, full raw body (including streamed SSE), write errors and completion time. Each response is associated with its zero-based request index. Snapshots also show actual request, completed-handler and unmatched-request counts; they make no claim that a client consumed a response.

A real App loopback provider returns more than 4 KiB of Unicode response bytes derived only from the received user request. The test proves exact response bytes survive evidence export. Process-isolated crash, protected Tool provenance and full response were exercised together. No product behavior, backend, source row or identity was synthesized.

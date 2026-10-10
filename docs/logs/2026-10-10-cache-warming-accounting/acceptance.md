# Acceptance

A fresh configuration makes no extra warming requests. Existing `idle` configuration is rejected with the allowed modes. To explicitly collect maintenance evidence on a model declaring warming support and cache lifetime, configure:

```yaml
runtime:
  cache_warming: streaming
  cache_warming_min_savings: 0
```

After a successful model call, inspect its Journal. The extra request has `source: maintenance`, its own call identity, usage evidence when reported, and one finish record. Its diagnostic joins that identity. Neither `Reply with one word: ok.` nor the discarded reply appears in conversation history. Ordinary answers retain their full transcript and trajectory step. Provider failure still closes the maintenance call; cancellation cancels the bounded request and settles before the run terminal. Mandatory accounting persistence failure must reach the producer rather than being classified as successful maintenance.

Use identical provider/model and `prefix_sha256` values when comparing warm attempts. Compare the actual output and cache buckets, including creation/write cost if a rate is declared. Unknown rates and unreported usage stay unknown; neither estimates nor a whole conversation's cache count prove savings for the reusable prefix. The deterministic fixture reports one 1,234-token cache write, then one 1,234-token read on the same exact prefix, with 1,345 prompt and two output tokens per request. It proves accounting behavior only. Enabling automatic warming would require a separate live, controlled cost/hit comparison; no such benefit is asserted by this delivery.

At a model-call/event admission ceiling, optional warming must skip rather than fail completed foreground work. A one-call run with `MaxModelCalls: 1` completes and makes no maintenance provider request. This does not excuse persistence or quota failures after a maintenance provider call has already been admitted. When a maintenance or warming-capable route lacks cache-write presence, cost remains unknown even with declared other prices and even after session route aggregation in either order.

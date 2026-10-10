# Complete cognitive inference requests

The host incorrectly applied the authored graph's 4 KiB task ceiling to a code-owned cognitive model envelope. Byte slicing discarded part of a valid JSON input or its output schema, and could split UTF-8. Preserve the complete envelope and use the existing `StartOneShotChild` admission boundary, which rejects tasks beyond the native 64 KiB bound before execution. No shared limit, public authored-graph ceiling, tool grant or model runtime changes.

The pinned Eino v0.9.13 capability check confirms `adk.AgentInput` uses the standard message input. The existing Service/child/engine path already carries bounded task text through those messages; no model constructor, parallel runner or direct model call is added. The sole Model adapter remains cognitiveModel over governed durable child Runs.

The actual App test now also supplies a 4,500-byte UTF-8 suffix plus a random user fact. It exposed a separate Laputa stage packet overflow after the request fix: raw evidence was carried beside its generated memory effect. The paired library repair removes that unused post-model batch while retaining complete candidates and sources. Both fixes are required for this real large-source flow.

This is local development, not final source-bound candidate acceptance. Arbitrary post-effect failures, request replay identity, remaining S03–S11 work, final full CI and sealing remain open.

# PLG-P7 internal moduleization summary

Date: 2026-09-12

Status: **DONE**

PLG-P7 now expresses required and optional internal organs through the same
Descriptor, Port, and generated Assembly mechanics used by the plugin
platform. The default Source Catalog selects canonical internal owners for
loop, model, storage, checkpoint, credential, sandbox, and optional Host
composition; shipped Recipes and SDK pack fixtures no longer use the former
`vivy/kernel` aggregate.

The extraction preserves existing authority boundaries. Service, Journal,
Policy, identity, RPC, ChannelHost, FaceHost, and ToolHost do not gain a
second owner. Eino imports remain confined to runtime/provider quarantine,
credential resolution is scoped and non-enumerable, and sandbox composition
does not replace Policy, approval, cancellation, or audit authority.

Generated Assembly lifecycle is transactional: startup and readiness are
ordered, failure rolls back in reverse order with the cause chain preserved,
and shutdown is reverse-order and idempotent. Conditional Host requirements
are enforced by the compiler, while omitted capabilities remain absent from
minimal assemblies.

This closes PLG-P7 only. PLG-P8 SCX integration and PLG-P9 release
conformance, Inspect, removal, and rollback remain separately scheduled work.

# Acceptance

A reviewer can compare the design, index and ORCH-01 through ORCH-08 and find the same approved identities and lifecycle: immutable origin parent Run, current authorizer Run, activation Run and ChildSession. Direct parent-child durable messaging is required, with stable identity, idempotent admission, order, lifecycle, safe consumption and at-least-once semantics. Context is explicit task, admitted direct messages and approved dependency outputs only; parent model only; selected tools may narrow but not widen the immutable ceiling.

The documents use verified Eino Workflow APIs, reject unreachable/unconsumed DAG nodes, retain the index as sole Story-state DAG, keep ORCH-01 as the only execution-ready Story, and require R1-R14 acceptance without bypassing G0/G1. PostgreSQL conformance is explicitly deferred, unverified and not represented as a pass.

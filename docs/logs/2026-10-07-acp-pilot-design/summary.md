# ACP restricted pilot design publication

Date: 2026-10-07 (Asia/Shanghai)
Related issue: [#1](https://github.com/ProjectViVy/agent-vivy/issues/1)
Branch: `ACP`
Base commit: `dd78fcf142f384d47ce5cfefb43738fdb9a7346d`

## Delivered

Published the [detailed design](../../superpowers/specs/2026-10-07-acp-stdio-face-design.md) for the owner-approved restricted ACP v1 pilot. It specifies the local stdio Face Provider, existing FaceHost authority path, session and cancellation state, ordered event projection, approvals and questions, output sanitization, resource limits, SDK compatibility risks, and 12 acceptance scenarios.

The pilot requires `mcpServers=[]` and does not claim full ACP baseline conformance. The document remains a review draft: G0 is open and G1 is not scheduled. Its publication changes only location metadata, Markdown line-break formatting, and the delivery-status paragraph from the reviewed standalone revision.

## Scope boundary

No runtime implementation, dependency pin, generated Assembly, canonical product contract, or product configuration changes are included. SDK compatibility, queue ordering, cancellation, shutdown and error sanitization still require executable G0 evidence. Canonical contract adoption and the implementation plan follow the design's G0 closure conditions.

This is a design publication, not a product release; no release record is needed.

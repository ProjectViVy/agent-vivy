# Native workflow proof exact-input closure

The Service-owned native Eino proof now carries its complete canonical original descriptor instead of a revision digest. The immutable Run user message remains authoritative. Workflow invocation and each node validate the complete fixed graph and selected task/approval values against that owner before tool-operation lookup, admission or effects.

The existing SQL operation owner already scopes node operations by RunID. Node IDs now use workflow-node-a/b. Complete original descriptor, selected node, task and approval intent remain private original operation arguments. Equal input replays the completed outcome; changed input conflicts under that same owner; claimed unknown effects stay fenced.

Approval interruption, persistence and resume bind actual original descriptor values and preserve Run/node/target checks. Eino NewWorkflow, AddInput/AddDependency, native checkpoint storage and resume remain unchanged. Descriptor format 2 marks the exact-input contract. Old format-1 Run descriptors and approval states lacking original descriptor values require explicit ErrWorkflowRecoveryRequired before checkpoint invocation or effects; historical checkpoint bytes remain intact. No SQL migration or old digest parser is added.

The workflow tool encodes the original model call ID reversibly. reply_parent encodes RunID and call ID as separate base64url components, so NUL or punctuation cannot collapse owner fields. Existing owner bounds (128 workflow / 256 child-mailbox bytes) reject oversized identities before admission, without truncation or hashed fallback. Normal provider call IDs fit these bounds.

Scope is native orchestration and direct tests, workflow/reply_parent tools and direct tests. This is implementation evidence, not the independent D14 audit. No release or standalone converter is produced; parent integration owns final candidate gates.

## Eino capability check

Pinned Eino v0.9.21 compose.NewWorkflow, WorkflowNode.AddInput/AddDependency, compose.WithCheckPointStore/WithCheckPointID, compose.ExtractInterruptInfo/ResumeWithData and tool.GetInterruptState/GetResumeContext already provide scheduling, suspension and resume. No custom runner or serializer is added. Existing registered native type names remain; the Run-owned descriptor format fences old invocation before native checkpoint use.

Real checkpoint tests exposed an Eino native slice/custom-JSON-method asymmetry for json.RawMessage (inspected in internal/serialization/serialization.go). The native request therefore carries ordinary []byte containing actual canonical JSON, while private approval arguments carry a nested descriptor object. Plain bytes round-trip through the existing codec; the real interruption/restart tests pass.

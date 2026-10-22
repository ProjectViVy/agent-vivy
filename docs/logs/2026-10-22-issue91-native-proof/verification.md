# Verification

Base a12748a7114bb54fd0bdf663bfb1f01fcec90802 was merged into the isolated D11 author branch. Tests ran in independent Linux-filesystem checkout /home/mastwet/work/vivy-issue91-d11-wf/agent-vivy with its own pinned Laputa 9acc999301d5. Process-local GOFLAGS=-p=1 avoided competing heavy builds while parent full CI was active.

RED before implementation:

- TestWorkflowOperationKeyRetainsOriginalCallIdentity failed: original call ID could not be recovered from the hash-derived key.
- TestParentReplyOperationKeyRetainsUnambiguousOwnerFields failed: original fields had no exact component boundary.
- TestOrchestrationChangedDescriptorCannotAllocateAnotherNodeEffect failed with a valid admitted context: after completing node B, changing its task and old revision identity returned success and allocated another effect under the same Run. An initial test-context setup error was corrected before this meaningful RED observation.

GREEN command:

```text
GOFLAGS=-p=1 go test ./internal/tools ./internal/runtime -run 'Orchestration|WorkflowOperationKey|ParentReply|WorkflowTool|WorkflowRejects' -count=1 -timeout 4m
```

PASS for both packages (tools 0.022s / runtime 1.111s before the usage interruption; repeated before the focused commit). Coverage includes overlapping native nodes and native checkpoint round-trip; Service approval resume; sibling completion/cancellation; completed-node replay; changed descriptor/task conflicts; claimed unknown-effect fence; format-1 recovery preserving checkpoint history; complete approval descriptor restoration; reversible identities; ambiguous delimiters; and over-bound rejection before runtime/mailbox admission.

An intermediate json.RawMessage native request field failed real checkpoint tests. Pinned Eino source inspection identified the slice/custom-marshaler asymmetry; ordinary []byte actual JSON fixed it without replacing native serialization.

Formatting and checks: gofmt on all seven changed Go files; git diff --check; staged diff --check. A scoped residue scan found no crypto/sha256, encoding/hex, RevisionDigest, revision_digest or operationDigest in the changed production closure. No production journal or tenant state was accessed.

Full CI, protocol/Face/browser, race, real PostgreSQL and native Windows acceptance are not claimed here. Parent integration owns those gates and independent review. No competing heavy gate was started.

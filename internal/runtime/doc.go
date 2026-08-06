// Package runtime is the Run service and the ONLY adapter to the Eino
// execution framework (github.com/cloudwego/eino/adk). It:
//
//   - builds the ChatModelAgent + Runner from a ProviderRef ChatModel;
//   - maps Eino AgentEvent stream deltas into Vivy domain.RunEvent values;
//   - coordinates tool interrupts with the approval flow and the two-layer
//     checkpoint bridge (D-028..D-030);
//   - enforces the exactly-one-terminal-event invariant (D-008);
//   - performs restart recovery of non-terminal runs (FR-8).
//
// Eino types stop at this package boundary. Nothing outside runtime and
// provider may import Eino.
//
// Skeleton stage: empty. Implemented in tasks C3/C4/C6/E1/E2.
package runtime

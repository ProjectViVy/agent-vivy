# Acceptance — CMP-1 clear offload

## How to verify manually

1. Start the development stack (`just run` + `cd ui; pnpm dev`, opening
   <http://127.0.0.1:3015>) and confirm that `config.yaml` sets
   `runtime.workspace_root` and enables `runtime.compaction`.
2. Have a continuous conversation with Vivy and let it make a dozen or more
   tool calls that produce large output (several KB, such as repeatedly reading
   large files), until the context crosses the compaction trigger line—the chat
   stream should show a "Context compacted"
   (`context.compacted`) event.
3. In the same session, ask for the details of an earlier tool call. The model
   should be able to call `read_file` (with a path such as
   `compaction/clear/<call-id>`, as stated in the compaction placeholder text),
   retrieve the original output, and answer. This is the core acceptance for
   this slice: cleared results are recoverable.
4. Open the workspace directory for that run (`<workspace_root>/<runID>/`):
   `compaction/clear/` should contain one file per cleared tool call, with the
   original tool output as its content.
5. A deployment without `workspace_root` behaves as before for compaction
   (placeholders are not offloaded), with no errors or behavioral difference.

## Regression surface

- The compaction-trigger and summarization paths (CMP-2 failover and usage
  events) are unaffected; the existing compaction contract tests are all green.
- The no-workspace case preserves the previous behavior byte-for-byte (`nil`
  backend → in-memory placeholder with no offload).

# VC-3 slice 5: read_file image support (tool-result parts envelope)

Date: 2026-09-01. Lane: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`).

## What changed

`read_file` now returns image files (png/jpeg/gif/webp) as multimodal tool
results instead of discarding the bytes as "binary", so vision models see
the actual image — the Crush read-image behavior, realized through the
eino-native multimodal surface rather than a homegrown format.

Kernel layers:

- `internal/tools`: `FileReadResult` gains `ImageMIME`/`ImageData`
  (`json:"-"` — the read tool never marshals them into the text result).
  `readFileTool` renders an image read as a `{"parts":[...]}` envelope:
  a text part naming the file/MIME/size plus an `image` part carrying the
  base64 bytes. The tool description tells the model images are attached
  for vision models.
- `internal/runtime/filesystem_backend.go`: extension-based `imageMIME`
  probe; an image read under the cap returns the raw bytes with
  `ImageMIME` set (no truncation — truncation would corrupt the image);
  an image over the read cap (default 1MB) fails loudly with the sizes
  named instead of silently truncating. The eino-native `einofs` `Read`
  guard reports image files with a pointer back to the `read_file` tool
  (it has no parts channel).
- `internal/runtime/tooladapter.go`: results shaped as a parts envelope
  bypass byte compaction — collapsing the head+tail of envelope JSON would
  corrupt it beyond parsing. The check strips the untrusted-header prefix
  first (the header is added before the probe).
- `internal/runtime/enhanced_tooladapter.go`: `normalizeEnhancedResult`
  now applies the byte budget **per text part** instead of whole-envelope:
  an over-budget text part is compacted (or replaced by a tombstone when
  its share is exhausted); media parts are exempt and stay bounded at
  their source (the read cap). Envelope-shaped results flow through
  `schema.ToolResult.Parts` to eino v0.9.13 `toolResultToBlocks`, verified
  in the module cache to preserve image parts to provider blocks.

Non-image reads are byte-identical to before: no `parts` field, same
numbered content, same truncation and binary handling.

## Boundaries (deliberate)

- Image sizing is bounded at the source (1MB default read cap), not by the
  32KB tool-result budget — the budget compacts text only.
- Only png/jpeg/gif/webp extensions are attached; other binaries keep the
  existing `{binary:true}` text result.
- The eino-native filesystem middleware still cannot surface images (no
  parts channel there); the `read_file` tool is the vision path, matching
  the slice-4 boundary style.
- Audio/video/file part types are parseable by the adapter (eino schema
  supports them) but no Vivy tool emits them yet.

## Crush alignment

Behavior/protocol alignment only; zero code copied (Crush is FSL-1.1-MIT).
Crush's read tool attaches images for vision models; Vivy reaches the same
model experience through the tool-result parts envelope on the eino-native
multimodal surface (per the no-homegrown list: multimodal reads use native eino).

## Explicitly not done

- `positionEncoding` negotiation stays on the VC-3 remaining list.
- No config knob for the image cap — it reuses `max_file_bytes`.

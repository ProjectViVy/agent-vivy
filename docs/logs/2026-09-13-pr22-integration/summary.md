# PR 22 integration summary

## Scope

- Integrated the merged PLG-P3 closure from PR #21 into the PLG-P7 internal
  moduleization branch.
- Resolved the plugin program status document so both PLG-P3 and PLG-P7 remain
  recorded as complete, while PLG-P8 and PLG-P9 remain unscheduled.
- Preserved custom `providers.*.env_key` configuration when composing the new
  scoped Credential Resolver. Provider Profile references and configured model
  references now enter the same non-enumerable `vivy/model` allow-list without
  reading or recording Secret values.

## Architecture review

- The generated Assembly lifecycle is started during application composition
  and closed during both failed composition and normal shutdown.
- Loop composition continues to adapt the repository-pinned Eino v0.9.13 ADK
  surface through `internal/runtime.EngineFactory`; no Eino type or import was
  added outside `internal/runtime` or `internal/provider`.
- Storage, checkpoint, model, credential, and sandbox composition remain on the
  existing Service, Journal, Policy, ToolHost, ChannelHost, FaceHost, and
  ActionHost paths. No second runtime or authority owner was introduced.

## Explicitly not done

- PLG-P8 SCX integration and PLG-P9 release conformance were not started.
- No Studio source, tenant Journal, or production workspace data was accessed.

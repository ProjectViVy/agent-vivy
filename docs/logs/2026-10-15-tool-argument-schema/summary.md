# A tool without arguments must still declare its argument object

## What changed

`internal/runtime/tooladapter.go` — `toolAdapter.Info` now always sets
`ParamsOneOf`. A tool whose spec declares neither a raw JSON `Schema` nor any
`Params` (a tool that takes no arguments) gets an empty **closed** object
schema:

```json
{"properties":{},"additionalProperties":false,"type":"object"}
```

`internal/runtime/tooladapter_test.go` — a regression test asserts the
invariant for one synthetic argument-less tool and for the four real tools that
were broken (`task_list`, `skills_list`, `enter_plan_mode`, `get_goal`): the
parameters must marshal to a JSON object of type `object` with
`additionalProperties: false`, never to `null`.

## Why

Eino treats a nil `ParamsOneOf` as "no input parameters"
(`schema/tool.go`: *"If ParamsOneOf is nil, the tool takes no input
parameters."*), but the OpenAI-compatible wire type in the pinned Eino adapter
has no `omitempty` on that field
(`eino-ext/libs/acl/openai@v0.1.17/tool.go`):

```go
type functionDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Parameters  *jsonschema.Schema `json:"parameters"`
}
```

So a parameter-less tool went out as `"parameters": null`. A strict
OpenAI-compatible endpoint rejects the **whole** request in that case, not just
the offending tool: the user's turn died with

```text
engine event error: [NodeRunError] error, status code: 422, status: 422
Unprocessable Entity, message: openai_error
node path: [node_1, ChatModel]
```

which the UI renders as the generic "this conversation could not be completed".
Measured directly against the gateway (see `verification.md`): `parameters`
omitted → 422, `parameters: null` → 422, `parameters: {}` → 200,
`{"type":"object","properties":{}}` → 200,
`{"type":"object","properties":{},"additionalProperties":false}` → 200.

Four tools declared no arguments, so any turn that carried one of them failed
regardless of provider, model or conversation: `task_list`, `skills_list`,
`enter_plan_mode`, `get_goal`. Two other argument-less tools
(`child_inbox`, `reply_parent`) had hand-declared the object schema, which is
evidence that the per-tool convention did not hold; the fix is in the single
conversion point so a new argument-less tool cannot reintroduce the defect.

## Scope

- Kernel only: `internal/runtime` (the Vivy `ToolSpec` -> Eino `ToolInfo`
  conversion seam).
- The declaration is truthful, not cosmetic: `emptyToolArgs` rejects any
  argument to these tools, so a closed object is the accurate schema.
- No provider, vendor-data, UI, RPC, or MCP change. A tool that declares a
  schema or params is untouched; the raw-schema path keeps its exact bytes.
- No second copy: the rule lives once, in the adapter, instead of a schema
  literal repeated per tool.

## Explicitly not done

- Not adding per-tool schema literals to the four tools (that is the duplication
  this fix removes).
- Not changing the Eino adapter or vendoring a patched Eino.
- Not touching `internal/modules/*` action schemas, which already declare their
  own empty object schemas.
- No backlog row: found and fixed in the same iteration.

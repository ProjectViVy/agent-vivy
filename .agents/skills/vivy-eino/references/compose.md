# Eino compose: Chain / Graph / Workflow / Runnable (v0.9.13)

Source: `compose/` in the local module cache. `compose` builds execution
pipelines from components and lambdas, compiles them into a `Runnable`, and
runs them in four modes.

## Runnable — the compiled product

```go
type Runnable[I, O any] interface {
    Invoke(ctx context.Context, input I, opts ...Option) (output O, err error)
    Stream(ctx context.Context, input I, opts ...Option) (output *schema.StreamReader[O], err error)
    Collect(ctx context.Context, input *schema.StreamReader[I], opts ...Option) (output O, err error)
    Transform(ctx context.Context, input *schema.StreamReader[I], opts ...Option) (output *schema.StreamReader[O], err error)
}
```

- `Invoke`: one input → one output (blocks).
- `Stream`: one input → stream of output chunks (caller closes the reader).
- `Collect`: stream of inputs → one output.
- `Transform`: stream of inputs → stream of outputs.
- The framework auto-derives missing modes (e.g. Invoke from Stream via concat), so implementing one mode often suffices.

## Chain — linear pipeline

```go
ch := compose.NewChain[map[string]any, []*schema.Message]()
ch.AppendChatTemplate(tpl)          // prompt.ChatTemplate
ch.AppendChatModel(chatModel)       // model.BaseChatModel
r, err := ch.Compile(ctx, compose.WithGraphName("qa"))
if err != nil { ... }
out, err := r.Invoke(ctx, map[string]any{"query": "what is eino?"})
```

Chain `Append*` methods (each returns the chain for chaining):

- `AppendChatTemplate(prompt.ChatTemplate)` / `AppendAgenticChatTemplate`
- `AppendChatModel(model.BaseChatModel)` / `AppendAgenticModel`
- `AppendToolsNode(*ToolsNode)` / `AppendAgenticToolsNode`
- `AppendLambda(*Lambda)` — arbitrary function node
- `AppendDocumentTransformer(document.Transformer)`, `AppendEmbedding`, `AppendRetriever`, `AppendLoader`, `AppendIndexer`
- `AppendParallel(*Parallel)`, `AppendBranch(*ChainBranch)`, `AppendGraph(AnyGraph)`, `AppendPassthrough()`

## Graph — DAG with branches and parallel

```go
g := compose.NewGraph[string, string]()
g.AddChatModelNode("model", chatModel)
g.AddToolsNode("tools", toolsNode)
g.AddLambdaNode("parse", compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
    return strings.TrimSpace(in), nil
}))
_ = g.AddEdge("model", "tools")
_ = g.AddEdge("tools", "parse")
r, err := g.Compile(ctx, compose.WithGraphName("agent_flow"))
out, _ := r.Invoke(ctx, "hello")
```

`Graph` node-add methods (all `func (g *graph) AddXxxNode(key string, node, opts ...GraphAddNodeOpt) error`):

- `AddChatModelNode` / `AddAgenticModelNode`
- `AddChatTemplateNode` / `AddAgenticChatTemplateNode`
- `AddToolsNode` / `AddAgenticToolsNode`
- `AddRetrieverNode` / `AddEmbeddingNode` / `AddLoaderNode` / `AddIndexerNode` / `AddDocumentTransformerNode`
- `AddLambdaNode(key, *Lambda)` / `AddGraphNode(key, AnyGraph)` / `AddPassthroughNode(key)`
- `AddEdge(startNode, endNode)`, `AddBranch(startNode, *GraphBranch)`

**GraphAddNodeOpt** (per node):

- `compose.WithNodeName(n)` — callback/observability name.
- `compose.WithInputKey(k)` / `compose.WithOutputKey(k)` — map a `map[string]any` input/output field to this node (essential when node input is `map[string]any`).
- `compose.WithStatePreHandler[I,S](fn)` / `compose.WithStatePostHandler[O,S](fn)` — touch shared graph state before/after node.

**GraphCompileOption**:

- `compose.WithGraphName(name)` — observable name.
- `compose.WithMaxRunSteps(n)` — cap super-steps (loop guard).
- `compose.WithNodeTriggerMode(compose.AllPredecessor | compose.AnyPredecessor)`.

**GraphBranch** — conditional routing:

```go
cond := func(ctx context.Context, in string) (string, error) {
    if strings.Contains(in, "weather") { return "weather_node", nil }
    return "default_node", nil
}
branch := compose.NewGraphBranch(cond, map[string]bool{"weather_node": true, "default_node": true})
_ = g.AddBranch("classify", branch) // after node "classify"
```

`NewGraphMultiBranch` returns multiple end nodes per condition.

## Workflow — declarative dependencies and field mapping

```go
wf := compose.NewWorkflow[string, string]()
modelNode := wf.AddChatModelNode("model", chatModel)   // AddXxxNode returns *WorkflowNode
parseNode := wf.AddLambdaNode("parse", lambda)          // same AddXxxNode set
parseNode.AddInput("model")                             // data flow: model output -> parse input
parseNode.AddDependency("other")                        // control-only dependency (no data)
r, err := wf.Compile(ctx)
```

- Under the hood it uses `NodeTriggerMode(AllPredecessor)`; cycles are not supported.
- `WorkflowNode.AddInput(fromNodeKey, inputs ...*FieldMapping)` — data edges with optional field mapping.
- `WorkflowNode.AddDependency(fromNodeKey)` — ordering only, no data.
- `WorkflowNode.SetStaticValue(path, value)` — inject constants.

## Lambda — arbitrary function nodes

```go
// one in, one out, no options
compose.InvokableLambda(func(ctx context.Context, in string) (string, error) { ... })

// streaming variants
compose.StreamableLambda(func(ctx context.Context, in string) (*schema.StreamReader[string], error) { ... })
compose.CollectableLambda(...)
compose.TransformableLambda(...)
```

`LambdaOpt`: `compose.WithLambdaCallbackEnable(bool)`, `compose.WithLambdaType(name)`.

## ToolsNode — execute tool calls in a pipeline

```go
toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
    Tools: []tool.BaseTool{t1, t2}, // must implement InvokableTool / StreamableTool
    // optional:
    ExecuteSequentially:     false,                       // default: run tool calls in parallel
    UnknownToolsHandler:     func(ctx, name, input) (string, error) {...}, // hallucinated tool names
    ToolArgumentsHandler:    func(ctx, name, args) (string, error) {...},
    ToolAliases:             map[string]compose.ToolAliasConfig{...},
    ToolCallMiddlewares:     []compose.ToolMiddleware{...},
})
```

Place it after a ChatModel node: model emits `ToolCalls`, ToolsNode executes them and emits tool-result messages.

## Parallel

```go
p := compose.NewParallel()
p.AddChatModelNode("m1", modelA)
p.AddChatModelNode("m2", modelB)
chain.AppendParallel(p) // or graph parallel via GraphBranch routing
```

## Field mapping (graph edges)

For `map[string]any`-typed nodes, remap fields on edges:

```go
_ = g.AddEdge("node1", "node2", &compose.FieldMapping{
    InputKey:  "a", // take node1 output field "a"
    OutputKey: "b", // feed as node2 input field "b"
})
```

## Live Vivy examples

- `internal/runtime/engine.go` builds an `adk.ChatModelAgent` with `adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: wrapped}}` — the compose graph is embedded in the ADK agent.
- `internal/runtime/modelbroker.go` uses `Generate`/`WithTools` directly (no graph) for bounded worker turns.
- `spike/einoverify/main.go` — standalone Eino usage spike covering model + compose.

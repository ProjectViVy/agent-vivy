# Eino RAG: Retriever / Embedding / Document / Indexer (v0.9.13)

Source: `components/{retriever,embedding,document,indexer}/` and `schema/document.go`
in the local module cache. RAG = retrieval-augmented generation: load documents,
split/transform them, embed them, index them, then retrieve relevant chunks at
query time and feed them into the ChatModel prompt.

## Data type: schema.Document

```go
type Document struct {
    Content  string         `json:"content"`
    MetaData map[string]any `json:"meta_data"`
}
// helpers: WithScore(score), Score(), WithDenseVector(v), DenseVector(),
// WithSparseVector(m), SparseVector(), WithExtraInfo(s), ExtraInfo(),
// WithSubIndexes(idx), SubIndexes(), WithDSLInfo(m), DSLInfo()
```

## Components

```go
// Loader: read raw docs from a source (file path or URL).
type Loader interface {
    Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) ([]*schema.Document, error)
}

// Transformer: split / filter / merge / re-rank documents.
type Transformer interface {
    Transform(ctx context.Context, src []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error)
}

// Embedder: turn texts into vectors.
type Embedder interface {
    EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) // invoke
    // + streaming variant
}

// Indexer: store documents into a vector/search index.
type Indexer interface {
    Index(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) // returns doc IDs
}

// Retriever: search relevant documents for a query.
type Retriever interface {
    Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error)
}
```

Retriever options (`retriever.WithIndex`, `WithSubIndex`, `WithTopK(n)`,
`WithScoreThreshold(f)`, `WithEmbedding(emb)`, `WithDSLInfo`).
Indexer options: `indexer.WithIndex(name)`.

## Typical RAG pipeline (compose)

```go
// Load → transform(split) → embed → index happens offline:
loader := ...        // e.g. a file/web loader
splitter := ...      // e.g. a document splitter (from eino-ext or custom Transformer)
embedder := ...      // e.g. an embedding model
indexer := ...       // e.g. a vector store indexer

ingestChain := compose.NewChain[document.Source, []string]()
ingestChain.AppendLoader(loader)
ingestChain.AppendDocumentTransformer(splitter)
ingestChain.AppendEmbedding(embedder)
ingestChain.AppendIndexer(indexer)
rIngest, err := ingestChain.Compile(ctx)

// Query time: retrieve → prompt → model
qChain := compose.NewChain[map[string]any, *schema.Message]()
qChain.AppendRetriever(retriever)          // query string in, docs out
qChain.AppendLambda(retriever2docsLambda)  // docs -> context string
qChain.AppendChatTemplate(chatTpl)         // builds messages incl. context
qChain.AppendChatModel(chatModel)
rQuery, err := qChain.Compile(ctx)
```

Note: concrete loaders/splitters/embedders/indexers/retrievers come from
`eino-ext` (per-provider packages) or your own implementations; the core
`components/*` packages only define the interfaces and options. Vivy currently
uses only `components/model` and `components/tool` from this set.

## Building a retriever-backed agent

Feed retrieval results into a ReAct agent by placing retrieval inside the
prompt assembly (message building) before the model, or by exposing retrieval
as a tool (see `references/adk.md`): the tool approach lets the model decide
when to search, which is the common agentic-RAG pattern in ADK.

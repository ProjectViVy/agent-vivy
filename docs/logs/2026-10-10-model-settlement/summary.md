# Model call settlement errors

The observed model wrappers now fail closed when `modelCallObserver.End`
cannot persist mandatory settlement. Generate and Stream setup preserve the
provider/Chunk cause and the settlement cause together. The stream pump settles
exactly once before exposing a terminal error; a complete provider response
whose settlement fails returns that error before EOF.

The existing `schema.Pipe(8)` producer barrier, usage sample, response-complete
flag, bound tools, observer factory, and wrapper routes remain authoritative.
A single pump finish path joins the call and settlement errors and closes the
writer; Eino's reader-close signal releases its error send and allows upstream
cleanup when the consumer leaves during settlement. Cache warming, maintenance
routes, pricing, and payload changes belong to separate delivery lanes.

## Eino capability check

Inspected the pinned Eino v0.9.13 APIs: `schema.Pipe`, `StreamWriter.Send`,
`StreamReader.Close`, `callbacks.OnEndWithStreamOutput`, and
`compose.genericOnEndWithStreamOutputHandle`. Callback handlers receive copied
streams after Stream returns and cannot replace the existing producer-path
persist-before-delivery barrier. This fix adapts the existing Eino model and
pipe seam without introducing another stream implementation. Revisit the
custom observation pump if Eino provides a producer-path Recv hook with
synchronous persistence, bounded backpressure, and fail-closed error delivery.

No release artifact or deployment is included; this delivery is a source fix.

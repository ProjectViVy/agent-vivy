package runtime

import (
	"context"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type modelStreamObserver struct {
	Begin func()
	Chunk func(*schema.Message) error
}

type modelStreamObserverKey struct{}

func withModelStreamObserver(ctx context.Context, observer modelStreamObserver) context.Context {
	return context.WithValue(ctx, modelStreamObserverKey{}, observer)
}

type observingChatModel struct {
	inner model.ToolCallingChatModel
}

func observeModelStreams(inner model.ToolCallingChatModel) model.ToolCallingChatModel {
	return &observingChatModel{inner: inner}
}

func (m *observingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.inner.Generate(ctx, input, opts...)
}

// Stream tees provider chunks onto a bounded pipe before Eino sees the
// reader. Inspected Eino v0.9.13 callbacks.OnEndWithStreamOutput /
// schema.StreamReader.Copy / compose.genericOnEndWithStreamOutput: those
// timings Copy the stream after inner Stream returns. A sync handler can
// run before compose Recv, but it is still a sibling copy — it cannot put
// persist on the producer Recv→Send path, apply Pipe(8) backpressure onto
// provider Recv, fail-close the graph copy from a persist error, or run
// the tool-settled barrier. Revisit if Eino adds a producer-path Recv hook
// with those semantics.
func (m *observingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	upstream, err := m.inner.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	observer, ok := ctx.Value(modelStreamObserverKey{}).(modelStreamObserver)
	if !ok || observer.Chunk == nil {
		return upstream, nil
	}
	// Mark the stream before exposing the tee to Eino. Starting the marker in
	// the pump goroutine races Eino's eager stream forwarding: the materialized
	// event can otherwise enter the mapper first and emit the same deltas again.
	if observer.Begin != nil {
		observer.Begin()
	}
	reader, writer := schema.Pipe[*schema.Message](8)
	go func() {
		defer upstream.Close()
		defer writer.Close()
		for {
			chunk, recvErr := upstream.Recv()
			if recvErr == io.EOF {
				return
			}
			if recvErr != nil {
				writer.Send(nil, recvErr)
				return
			}
			if err := observer.Chunk(chunk); err != nil {
				writer.Send(nil, err)
				return
			}
			if writer.Send(chunk, nil) {
				return
			}
		}
	}()
	return reader, nil
}

func (m *observingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &observingChatModel{inner: bound}, nil
}

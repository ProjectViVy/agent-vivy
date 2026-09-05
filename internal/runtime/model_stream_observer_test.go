package runtime

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type pipeChatModel struct {
	reader *schema.StreamReader[*schema.Message]
	err    error
}

func (m *pipeChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("generate unused")
}

func (m *pipeChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.reader, nil
}

func (m *pipeChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestObservingChatModelBeginRunsBeforeStreamReturns(t *testing.T) {
	upstream, writer := schema.Pipe[*schema.Message](1)
	defer writer.Close()
	began := false
	ctx := withModelStreamObserver(context.Background(), modelStreamObserver{
		Begin: func() { began = true },
		Chunk: func(*schema.Message) error { return nil },
	})
	reader, err := observeModelStreams(&pipeChatModel{reader: upstream}).Stream(ctx, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()
	if !began {
		t.Fatal("Begin must run before Stream returns")
	}
}

func TestObservingChatModelObservesChunkBeforeDownstreamRecv(t *testing.T) {
	upstream, writer := schema.Pipe[*schema.Message](1)
	observed := make(chan *schema.Message, 1)
	ctx := withModelStreamObserver(context.Background(), modelStreamObserver{
		Chunk: func(chunk *schema.Message) error {
			observed <- chunk
			return nil
		},
	})
	reader, err := observeModelStreams(&pipeChatModel{reader: upstream}).Stream(ctx, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()

	want := &schema.Message{Role: schema.Assistant, Content: "hello"}
	if writer.Send(want, nil) {
		t.Fatal("upstream closed before send")
	}
	writer.Close()

	var got *schema.Message
	select {
	case got = <-observed:
	case <-time.After(time.Second):
		t.Fatal("chunk was not observed before downstream Recv")
	}
	if got != want {
		t.Fatalf("observed %+v, want %+v", got, want)
	}
	chunk, recvErr := reader.Recv()
	if recvErr != nil {
		t.Fatalf("downstream recv: %v", recvErr)
	}
	if chunk != want {
		t.Fatalf("downstream %+v, want %+v", chunk, want)
	}
}

func TestObservingChatModelChunkErrorFailsDownstream(t *testing.T) {
	upstream, writer := schema.Pipe[*schema.Message](1)
	fail := errors.New("persist failed")
	ctx := withModelStreamObserver(context.Background(), modelStreamObserver{
		Chunk: func(*schema.Message) error { return fail },
	})
	reader, err := observeModelStreams(&pipeChatModel{reader: upstream}).Stream(ctx, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()

	writer.Send(&schema.Message{Role: schema.Assistant, Content: "x"}, nil)
	writer.Close()

	_, recvErr := reader.Recv()
	if !errors.Is(recvErr, fail) {
		t.Fatalf("downstream err = %v, want %v", recvErr, fail)
	}
}

func TestObservingChatModelWithoutObserverReturnsUpstream(t *testing.T) {
	upstream, writer := schema.Pipe[*schema.Message](1)
	defer writer.Close()
	reader, err := observeModelStreams(&pipeChatModel{reader: upstream}).Stream(context.Background(), nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if reader != upstream {
		t.Fatal("missing observer must return the inner stream unchanged")
	}
}

func TestObservingChatModelInnerStreamError(t *testing.T) {
	innerErr := errors.New("provider stream")
	_, err := observeModelStreams(&pipeChatModel{err: innerErr}).Stream(context.Background(), nil)
	if !errors.Is(err, innerErr) {
		t.Fatalf("err = %v, want %v", err, innerErr)
	}
}

func TestObservingChatModelUpstreamRecvErrorFailsDownstream(t *testing.T) {
	upstream, writer := schema.Pipe[*schema.Message](1)
	fail := errors.New("provider recv")
	ctx := withModelStreamObserver(context.Background(), modelStreamObserver{
		Chunk: func(*schema.Message) error { return nil },
	})
	reader, err := observeModelStreams(&pipeChatModel{reader: upstream}).Stream(ctx, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()
	writer.Send(nil, fail)
	writer.Close()
	_, recvErr := reader.Recv()
	if !errors.Is(recvErr, fail) {
		t.Fatalf("downstream err = %v, want %v", recvErr, fail)
	}
}

func TestObservingChatModelWithToolsKeepsTee(t *testing.T) {
	upstream, writer := schema.Pipe[*schema.Message](1)
	defer writer.Close()
	bound, err := observeModelStreams(&pipeChatModel{reader: upstream}).WithTools(nil)
	if err != nil {
		t.Fatalf("with tools: %v", err)
	}
	if _, ok := bound.(*observingChatModel); !ok {
		t.Fatalf("bound type %T, want *observingChatModel", bound)
	}
	observed := make(chan struct{}, 1)
	ctx := withModelStreamObserver(context.Background(), modelStreamObserver{
		Chunk: func(*schema.Message) error {
			observed <- struct{}{}
			return nil
		},
	})
	reader, err := bound.Stream(ctx, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()
	writer.Send(&schema.Message{Role: schema.Assistant, Content: "x"}, nil)
	select {
	case <-observed:
	case <-time.After(time.Second):
		t.Fatal("WithTools-bound model did not tee")
	}
}

func TestObservingChatModelAppliesBoundedBackpressure(t *testing.T) {
	upstream, writer := schema.Pipe[*schema.Message](0)
	ctx := withModelStreamObserver(context.Background(), modelStreamObserver{
		Chunk: func(*schema.Message) error { return nil },
	})
	reader, err := observeModelStreams(&pipeChatModel{reader: upstream}).Stream(ctx, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()

	const want = 16
	sent := make(chan int, 1)
	go func() {
		n := 0
		for n < want {
			if writer.Send(&schema.Message{Role: schema.Assistant, Content: "x"}, nil) {
				break
			}
			n++
		}
		sent <- n
	}()
	select {
	case n := <-sent:
		t.Fatalf("provider sent %d chunks without downstream Recv; Pipe(8) should stall Recv", n)
	case <-time.After(200 * time.Millisecond):
	}
	go func() {
		for {
			if _, recvErr := reader.Recv(); recvErr != nil {
				return
			}
		}
	}()
	select {
	case n := <-sent:
		if n != want {
			t.Fatalf("after drain sent %d, want %d", n, want)
		}
	case <-time.After(time.Second):
		t.Fatal("provider send loop did not unblock after drain")
	}
	writer.Close()
}

func TestObservingChatModelClosesOnEOF(t *testing.T) {
	upstream, writer := schema.Pipe[*schema.Message](1)
	ctx := withModelStreamObserver(context.Background(), modelStreamObserver{
		Chunk: func(*schema.Message) error { return nil },
	})
	reader, err := observeModelStreams(&pipeChatModel{reader: upstream}).Stream(ctx, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer reader.Close()
	writer.Close()
	if _, recvErr := reader.Recv(); recvErr != io.EOF {
		t.Fatalf("recv = %v, want EOF", recvErr)
	}
}

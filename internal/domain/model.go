package domain

import (
	"context"
	"io"
)

// ChatModel is the only model shape the runtime needs. It is satisfied
// by the mock provider (C1) and by real providers behind internal/provider
// (C2); the Eino adapter seam lives in internal/runtime. Defined here so
// domain never imports Eino (D-007).
type ChatModel interface {
	Stream(ctx context.Context, input []*Message) (Stream[*Message], error)
}

// Stream is a pull-style iterator. Recv returns io.EOF once the stream
// is exhausted.
type Stream[T any] interface {
	Recv() (T, error)
}

// Collect drains a stream into a slice. It is a convenience for tests
// and adapters; production paths consume Recv directly so cancellation
// stays responsive.
func Collect[T any](s Stream[T]) ([]T, error) {
	var out []T
	for {
		v, err := s.Recv()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, v)
	}
}

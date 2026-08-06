package provider

import (
	"context"
	"io"

	"agent-vivy/internal/domain"
)

// mockChunkBytes is the fixed delta size of the mock stream. Fixed size
// (not random) keeps runs reproducible (FR-3, NFR deterministic mode).
const mockChunkBytes = 8

// Mock is the deterministic chat provider for tests and offline
// development. The reply is a pure function of the input: no randomness,
// no network, no clock — the same input always yields the same chunk
// sequence (FR-3).
type Mock struct{}

// NewMock returns the deterministic mock provider.
func NewMock() *Mock { return &Mock{} }

// Stream produces the scripted reply as fixed-size deltas. The last user
// message drives the reply text; ctx is honored on every Recv so
// cancellation stays responsive.
func (m *Mock) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lastUser := ""
	for i := len(input) - 1; i >= 0; i-- {
		if input[i] != nil && input[i].Role == domain.RoleUser {
			lastUser = input[i].Content
			break
		}
	}

	reply := mockReply(lastUser)
	chunks := make([]*domain.Message, 0, len(reply)/mockChunkBytes+1)
	runes := []rune(reply)
	for off := 0; off < len(runes); off += mockChunkBytes {
		end := off + mockChunkBytes
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, &domain.Message{
			Role:    domain.RoleAssistant,
			Content: string(runes[off:end]),
		})
	}
	return &mockStream{chunks: chunks}, nil
}

// mockReply is a pure function of the user text; never randomized.
func mockReply(userText string) string {
	if userText == "" {
		return "mock: empty input acknowledged."
	}
	return "mock reply to: " + userText
}

// mockStream replays the pre-computed chunk list.
type mockStream struct {
	chunks []*domain.Message
	next   int
}

func (s *mockStream) Recv() (*domain.Message, error) {
	if s.next >= len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.next]
	s.next++
	return chunk, nil
}

// Package testsupport contains deterministic model doubles used only by
// repository tests. It is deliberately outside internal/provider so test
// behavior cannot be selected by a product configuration or Catalog lookup.
package testsupport

import (
	"context"
	"io"

	"agent-vivy/internal/domain"
)

// EchoModel returns a deterministic text response for the last user message.
// It is a test fixture, not a product provider.
type EchoModel struct{}

// NewEchoModel constructs the deterministic test model.
func NewEchoModel() *EchoModel { return &EchoModel{} }

func (EchoModel) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lastUser := ""
	for index := len(input) - 1; index >= 0; index-- {
		if input[index] != nil && input[index].Role == domain.RoleUser {
			lastUser = input[index].Content
			break
		}
	}
	return &echoStream{message: &domain.Message{Role: domain.RoleAssistant, Content: "test response to: " + lastUser}}, nil
}

type echoStream struct {
	message *domain.Message
	done    bool
}

func (s *echoStream) Recv() (*domain.Message, error) {
	if s.done {
		return nil, io.EOF
	}
	s.done = true
	return s.message, nil
}

var _ domain.ChatModel = (*EchoModel)(nil)
var _ domain.Stream[*domain.Message] = (*echoStream)(nil)

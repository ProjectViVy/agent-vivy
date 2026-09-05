package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const (
	maxThoughtsPerRun = 32
	maxThoughtBytes   = 16 << 10
)

type SequentialThinkingBackend struct {
	mu   sync.Mutex
	runs map[domain.RunID]*thinkingRun
}

type thinkingRun struct {
	total    int
	last     int
	thoughts map[int]string
	branches map[string]int
}

var _ tools.SequentialThinkingOperations = (*SequentialThinkingBackend)(nil)

func NewSequentialThinkingBackend() *SequentialThinkingBackend {
	return &SequentialThinkingBackend{runs: make(map[domain.RunID]*thinkingRun)}
}
func (b *SequentialThinkingBackend) Think(_ context.Context, runID domain.RunID, request tools.SequentialThoughtRequest) (tools.SequentialThoughtResponse, error) {
	thought := strings.TrimSpace(request.Thought)
	if thought == "" {
		return tools.SequentialThoughtResponse{}, errors.New("sequential thinking: thought must not be empty")
	}
	if len(thought) > maxThoughtBytes {
		return tools.SequentialThoughtResponse{}, fmt.Errorf("sequential thinking: thought exceeds %d bytes", maxThoughtBytes)
	}
	if request.ThoughtNumber < 1 || request.ThoughtNumber > maxThoughtsPerRun {
		return tools.SequentialThoughtResponse{}, fmt.Errorf("sequential thinking: thought_number must be between 1 and %d", maxThoughtsPerRun)
	}
	if request.TotalThoughts < request.ThoughtNumber || request.TotalThoughts > maxThoughtsPerRun {
		return tools.SequentialThoughtResponse{}, fmt.Errorf("sequential thinking: total_thoughts must be between thought_number and %d", maxThoughtsPerRun)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.runs[runID]
	if state == nil {
		state = &thinkingRun{thoughts: make(map[int]string), branches: make(map[string]int)}
		b.runs[runID] = state
	}
	if request.IsRevision {
		if request.RevisesThought < 1 || request.RevisesThought > state.last || state.thoughts[request.RevisesThought] == "" {
			return tools.SequentialThoughtResponse{}, errors.New("sequential thinking: revision target does not exist")
		}
	} else if request.BranchFromThought > 0 {
		if request.BranchFromThought > state.last || state.thoughts[request.BranchFromThought] == "" {
			return tools.SequentialThoughtResponse{}, errors.New("sequential thinking: branch target does not exist")
		}
	} else if request.ThoughtNumber != state.last+1 {
		return tools.SequentialThoughtResponse{}, fmt.Errorf("sequential thinking: expected thought_number %d", state.last+1)
	}
	if state.total > 0 && request.TotalThoughts < state.total {
		request.TotalThoughts = state.total
	}
	state.total = request.TotalThoughts
	if request.ThoughtNumber > state.last {
		state.last = request.ThoughtNumber
	}
	state.thoughts[request.ThoughtNumber] = thought
	if request.BranchID != "" {
		state.branches[request.BranchID] = request.ThoughtNumber
	}
	return tools.SequentialThoughtResponse{Thought: thought, ThoughtNumber: request.ThoughtNumber, TotalThoughts: state.total, NextThoughtNeeded: request.NextThoughtNeeded, IsRevision: request.IsRevision, BranchID: request.BranchID, Accepted: true}, nil
}

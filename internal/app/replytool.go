package app

import (
	"context"
	"errors"
	"sync"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
)

type replyParentToolRef struct {
	mu      sync.Mutex
	manager *workerManager
}

func (r *replyParentToolRef) arm(manager *workerManager) {
	r.mu.Lock()
	r.manager = manager
	r.mu.Unlock()
}

func (r *replyParentToolRef) SendParentMessage(ctx context.Context, runID domain.RunID, sessionID domain.SessionID, operationKey, text string) (tools.ParentMessageResult, error) {
	r.mu.Lock()
	manager := r.manager
	r.mu.Unlock()
	if manager == nil || manager.service == nil {
		return tools.ParentMessageResult{}, errors.New("child messaging is not wired")
	}
	if tools.RunIDFromContext(ctx) != runID || tools.SessionIDFromContext(ctx) != sessionID {
		return tools.ParentMessageResult{}, errors.New("reply_parent context does not match the active child")
	}
	message, _, err := manager.service.SendChildMessage(ctx, runtime.ChildMessageSendRequest{
		ChildSessionID: sessionID, AuthorizerRunID: runID, IdempotencyKey: operationKey, Body: []byte(text),
	})
	if err != nil {
		return tools.ParentMessageResult{}, err
	}
	return tools.ParentMessageResult{MessageID: message.ID, Sequence: message.Sequence, Status: string(message.Status)}, nil
}

func (r *replyParentToolRef) ReadParentInbox(ctx context.Context, runID domain.RunID, sessionID domain.SessionID) ([]tools.ChildInboxMessage, error) {
	r.mu.Lock()
	manager := r.manager
	r.mu.Unlock()
	if manager == nil || manager.service == nil {
		return nil, errors.New("child inbox is not wired")
	}
	if tools.RunIDFromContext(ctx) != runID || tools.SessionIDFromContext(ctx) != sessionID {
		return nil, errors.New("child_inbox context does not match the active parent")
	}
	return manager.service.ReadParentInbox(ctx, runID, sessionID)
}

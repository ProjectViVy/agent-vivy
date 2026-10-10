//go:build diva_c06_overlay

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	mentlefacade "github.com/dashimaki/mentle/facade"
)

func memoryLoopC06OverlayAvailable() bool { return true }

func configureMemoryLoopC06Hook(path string) {
	if path == "" {
		return
	}
	mentlefacade.SetC06TestBeforeIndexApply(func(ctx context.Context, receipt mentlefacade.MutationReceipt) {
		if !strings.Contains(receipt.OperationID, "/graph/nodes/effects:") {
			return
		}
		claim, err := os.OpenFile(path+".claimed", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			return // Restart consumes the outbox job without blocking again.
		}
		if err != nil {
			panic(fmt.Errorf("claim C06 handshake: %w", err))
		}
		if err := claim.Close(); err != nil {
			panic(fmt.Errorf("close C06 handshake claim: %w", err))
		}
		handshake := memoryLoopC06Handshake{
			PID: os.Getpid(), OperationID: receipt.OperationID, TargetRef: receipt.RecordID,
			Revision: uint64(receipt.Revision), Status: receipt.Status,
			CanonicalStatus: receipt.CanonicalStatus, IndexStatus: receipt.IndexStatus,
		}
		raw, err := json.Marshal(handshake)
		if err != nil {
			panic(fmt.Errorf("encode C06 handshake: %w", err))
		}
		temporary := fmt.Sprintf("%s.tmp-%d", path, os.Getpid())
		if err := os.WriteFile(temporary, raw, 0600); err != nil {
			panic(fmt.Errorf("write C06 handshake: %w", err))
		}
		if err := os.Rename(temporary, path); err != nil {
			panic(fmt.Errorf("publish C06 handshake: %w", err))
		}
		<-ctx.Done() // Parent kills the process before the derived index is applied.
	})
}

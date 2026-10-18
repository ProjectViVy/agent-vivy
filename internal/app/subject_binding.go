package app

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
)

// ResolveSubjectBinding registers the organism once in its existing core
// SnapshotStore. It runs against the actual ACTMEM Root, before owner ingestion.
// Neither presentation changes nor another configured path can rekey it.
func ResolveSubjectBinding(ctx context.Context, store storage.SnapshotStore, ownerPath string) (storage.SubjectHistoryBinding, error) {
	if store == nil || ownerPath == "" {
		return storage.SubjectHistoryBinding{}, storage.ErrSubjectBinding
	}
	canonical, err := canonicalOwnerPath(ownerPath)
	if err != nil {
		return storage.SubjectHistoryBinding{}, err
	}
	for {
		raw, version, err := store.Get(ctx, domain.SubjectIdentityKey)
		if err != nil {
			return storage.SubjectHistoryBinding{}, err
		}
		if version != 0 {
			var id domain.SubjectIdentity
			if err := json.Unmarshal(raw, &id); err != nil {
				return storage.SubjectHistoryBinding{}, fmt.Errorf("%w: invalid identity record", storage.ErrSubjectBinding)
			}
			if !id.Valid() || id.OwnerPath != canonical {
				return storage.SubjectHistoryBinding{}, storage.ErrSubjectBinding
			}
			return storage.SubjectHistoryBinding{SubjectID: id.SubjectID, DomainID: id.DomainID}, nil
		}
		if len(raw) != 0 {
			return storage.SubjectHistoryBinding{}, storage.ErrSubjectBinding
		}
		id := domain.SubjectIdentity{Version: 1, SubjectID: uuid.NewString(), DomainID: domain.SubjectPersonalDomain, AuthorityID: uuid.NewString(), OwnerPath: canonical}
		encoded, err := json.Marshal(id)
		if err != nil {
			return storage.SubjectHistoryBinding{}, err
		}
		if err = store.Put(ctx, domain.SubjectIdentityKey, encoded, 0); errors.Is(err, storage.ErrVersionConflict) {
			continue
		} else if err != nil {
			return storage.SubjectHistoryBinding{}, err
		}
		return storage.SubjectHistoryBinding{SubjectID: id.SubjectID, DomainID: id.DomainID}, nil
	}
}

// Resolve existing ancestors as well: first boot can register a directory
// before it exists without letting a symlinked parent create a second owner.
func canonicalOwnerPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for current := absolute; ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(current) == current {
			return "", fmt.Errorf("app: resolve ACTMEM owner: %w", err)
		}
		suffix = append(suffix, filepath.Base(current))
	}
}

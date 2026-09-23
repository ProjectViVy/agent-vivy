package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"agent-vivy/internal/storage"
)

// Fail-closed read errors: a checkpoint that cannot be verified is never
// handed back to the engine (docs/eino-capability-verify.md §3.5).
var (
	// ErrCheckpointCorrupted means the stored payload no longer matches
	// the checksum recorded in the Vivy envelope.
	ErrCheckpointCorrupted = errors.New("runtime: checkpoint blob is corrupted")
	// ErrCheckpointEngineVersionMismatch means the checkpoint was written
	// by a different eino build; eino's checkpoint format carries no
	// compatibility promise, so reads refuse it outright.
	ErrCheckpointEngineVersionMismatch = errors.New("runtime: checkpoint was written by a different engine version")
	// ErrCheckpointPromptMismatch means the durable opaque checkpoint is not
	// bound to the immutable prompt snapshot that the caller supplied for the
	// run. It is deliberately fail-closed: no new model/tool continuation may
	// consume an unbound checkpoint.
	ErrCheckpointPromptMismatch = errors.New("runtime: checkpoint prompt identity mismatch")
)

const einoModulePath = "github.com/cloudwego/eino"

// checkpointEnvelope is the Vivy header wrapped around the opaque eino
// checkpoint bytes. On-disk layout: 4-byte big-endian header length,
// header JSON, then the raw payload.
type checkpointEnvelope struct {
	EngineVersion  string                    `json:"engine_version"`
	ChecksumSHA256 string                    `json:"checksum_sha256"`
	CreatedAt      int64                     `json:"created_at"`
	Prompt         *checkpointPromptIdentity `json:"prompt,omitempty"`
}

type checkpointPromptIdentity struct {
	RunID           string `json:"run_id"`
	SchemaVersion   int    `json:"schema_version"`
	ComposerVersion string `json:"composer_version"`
	GenerationID    string `json:"generation_id"`
	PayloadSHA256   string `json:"payload_sha256"`
}

// VersionedCheckpointStore is the outer half of the two-layer checkpoint
// bridge (C6): the eino runner treats checkpoint bytes as opaque, this
// store wraps them in a Vivy envelope carrying the engine version and a
// payload checksum before committing to the generation-based
// storage.BlobStore, and verifies both on read (fail-closed).
type VersionedCheckpointStore struct {
	blobs         storage.BlobStore
	engineVersion string
}

// NewVersionedCheckpointStore wires the bridge over a blob store.
// engineVersion must be the eino module version the binary was built
// with (see EinoEngineVersion); checkpoints written under any other
// version fail to load.
func NewVersionedCheckpointStore(blobs storage.BlobStore, engineVersion string) (*VersionedCheckpointStore, error) {
	if blobs == nil {
		return nil, errors.New("runtime: nil blob store")
	}
	if engineVersion == "" {
		return nil, errors.New("runtime: empty engine version")
	}
	return &VersionedCheckpointStore{blobs: blobs, engineVersion: engineVersion}, nil
}

// Set wraps payload in the Vivy envelope and commits it as a new blob
// generation (D-030).
func (s *VersionedCheckpointStore) Set(ctx context.Context, id string, payload []byte) error {
	identity, err := checkpointPromptFor(ctx, id)
	if err != nil {
		return err
	}
	blob, err := encodeCheckpointEnvelope(s.engineVersion, payload, identity)
	if err != nil {
		return err
	}
	if err := s.blobs.Put(ctx, id, blob); err != nil {
		return fmt.Errorf("runtime: persist checkpoint %q: %w", id, err)
	}
	return nil
}

// Get loads the current generation, verifies checksum and engine version,
// and returns the inner eino bytes. Absent ids yield (nil, false, nil);
// any verification failure is a fail-closed error.
func (s *VersionedCheckpointStore) Get(ctx context.Context, id string) ([]byte, bool, error) {
	blob, ok, err := s.blobs.Get(ctx, id)
	if !ok || err != nil {
		return nil, ok, err
	}
	payload, env, err := decodeCheckpointEnvelope(blob)
	if err != nil {
		return nil, false, err
	}
	if env.EngineVersion != s.engineVersion {
		return nil, false, fmt.Errorf("%w: stored %q, current %q", ErrCheckpointEngineVersionMismatch, env.EngineVersion, s.engineVersion)
	}
	if err := verifyCheckpointPrompt(ctx, id, env.Prompt); err != nil {
		return nil, false, err
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != env.ChecksumSHA256 {
		return nil, false, fmt.Errorf("%w: %q", ErrCheckpointCorrupted, id)
	}
	return payload, true, nil
}

// Delete removes the checkpoint and all of its generations.
func (s *VersionedCheckpointStore) Delete(ctx context.Context, id string) error {
	return s.blobs.Delete(ctx, id)
}

func encodeCheckpointEnvelope(engineVersion string, payload []byte, prompt *checkpointPromptIdentity) ([]byte, error) {
	sum := sha256.Sum256(payload)
	env := checkpointEnvelope{
		EngineVersion:  engineVersion,
		ChecksumSHA256: hex.EncodeToString(sum[:]),
		CreatedAt:      time.Now().UnixMilli(),
		Prompt:         prompt,
	}
	header, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("runtime: encode checkpoint envelope: %w", err)
	}
	blob := make([]byte, 0, 4+len(header)+len(payload))
	blob = binary.BigEndian.AppendUint32(blob, uint32(len(header)))
	blob = append(blob, header...)
	blob = append(blob, payload...)
	return blob, nil
}

func checkpointPromptFor(ctx context.Context, id string) (*checkpointPromptIdentity, error) {
	snapshot, ok := runPrompt(ctx)
	if !ok {
		return nil, nil
	}
	if err := validateCheckpointPromptSnapshot(snapshot, id); err != nil {
		return nil, err
	}
	return &checkpointPromptIdentity{
		RunID:           string(snapshot.RunID),
		SchemaVersion:   snapshot.SchemaVersion,
		ComposerVersion: snapshot.ComposerVersion,
		GenerationID:    snapshot.GenerationID,
		PayloadSHA256:   snapshot.PayloadSHA256,
	}, nil
}

func verifyCheckpointPrompt(ctx context.Context, id string, stored *checkpointPromptIdentity) error {
	snapshot, hasSnapshot := runPrompt(ctx)
	if !hasSnapshot {
		if stored != nil {
			return fmt.Errorf("%w: checkpoint %q requires its prompt snapshot", ErrCheckpointPromptMismatch, id)
		}
		return nil
	}
	if err := validateCheckpointPromptSnapshot(snapshot, id); err != nil {
		return err
	}
	if stored == nil {
		return fmt.Errorf("%w: checkpoint %q has no prompt identity", ErrCheckpointPromptMismatch, id)
	}
	want := checkpointPromptIdentity{
		RunID: string(snapshot.RunID), SchemaVersion: snapshot.SchemaVersion,
		ComposerVersion: snapshot.ComposerVersion, GenerationID: snapshot.GenerationID,
		PayloadSHA256: snapshot.PayloadSHA256,
	}
	if *stored != want {
		return fmt.Errorf("%w: checkpoint %q does not match run %q", ErrCheckpointPromptMismatch, id, snapshot.RunID)
	}
	return nil
}

func validateCheckpointPromptSnapshot(snapshot storage.RunPromptSnapshot, id string) error {
	if _, err := storage.ValidateRunPromptSnapshot(snapshot); err != nil {
		return fmt.Errorf("%w: invalid prompt snapshot: %v", ErrCheckpointPromptMismatch, err)
	}
	if snapshot.SchemaVersion != promptSchemaVersion || snapshot.ComposerVersion != promptComposerVersion {
		return fmt.Errorf("%w: unsupported prompt schema or composer", ErrCheckpointPromptMismatch)
	}
	if snapshot.RunID == "" || id != checkpointIDFor(snapshot.RunID) {
		return fmt.Errorf("%w: checkpoint %q is not owned by run %q", ErrCheckpointPromptMismatch, id, snapshot.RunID)
	}
	return nil
}

func decodeCheckpointEnvelope(blob []byte) (payload []byte, env checkpointEnvelope, err error) {
	if len(blob) < 4 {
		return nil, env, fmt.Errorf("%w: truncated header length", ErrCheckpointCorrupted)
	}
	n := binary.BigEndian.Uint32(blob[:4])
	if uint32(len(blob)-4) < n {
		return nil, env, fmt.Errorf("%w: truncated header", ErrCheckpointCorrupted)
	}
	if err := json.Unmarshal(blob[4:4+n], &env); err != nil {
		return nil, env, fmt.Errorf("%w: %v", ErrCheckpointCorrupted, err)
	}
	return blob[4+n:], env, nil
}

// engineVersionOverride pins the eino version when build-info metadata is
// unavailable. Release binaries get it from the embedded module info; go
// test binaries lack it, so suites pin the version they were built
// against (the go.mod pin) via SetEngineVersionOverride.
var engineVersionOverride string

// SetEngineVersionOverride pins the reported eino version for binaries
// without embedded module metadata (go test suites). Empty restores the
// build-info lookup. Production binaries never call this.
func SetEngineVersionOverride(version string) { engineVersionOverride = version }

// EinoEngineVersion reports the eino module version embedded in the
// binary's build info, or "" when unavailable (source builds without
// module metadata). App wiring must treat "" as a startup failure: an
// unknown version cannot anchor the fail-closed checkpoint contract.
func EinoEngineVersion() string {
	if engineVersionOverride != "" {
		return engineVersionOverride
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range bi.Deps {
		if dep.Path == einoModulePath {
			return dep.Version
		}
	}
	return ""
}

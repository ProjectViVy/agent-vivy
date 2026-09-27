package moduleport

import (
	"context"

	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

// MaskDependencies is the only construction input that the build-owned mask
// Provider may receive. Generation identity binds immutable built-ins, while
// storage owns custom definitions, selections, and admission checks.
type MaskDependencies struct {
	Store        storage.MaskStore
	GenerationID string
}

// MaskFactory is emitted into a generated RuntimeAssembly only when the
// closed mask-service Port is selected. Keeping this signature in the
// module-port layer avoids a maskcontract -> storage dependency cycle.
type MaskFactory func(context.Context, MaskDependencies) (maskcontract.Service, error)

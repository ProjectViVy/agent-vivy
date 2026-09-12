package modules

import (
	"context"

	"agent-vivy/sdk/module"
)

type Instance = module.Instance
type Generation = module.Generation

func Start(ctx context.Context, owners []module.Instance) (*Generation, error) {
	return module.StartGeneration(ctx, owners)
}

func CloseConstructed(ctx context.Context, owners []module.Instance) error {
	return module.CloseConstructed(ctx, owners)
}

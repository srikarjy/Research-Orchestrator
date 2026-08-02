package analyzers

import (
	"context"
)

type Analyzer interface {
	Name() string
	Category() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}
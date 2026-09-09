package sync

import (
	"context"
	"sync"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// Transformer post-processes a freshly fetched snapshot before it's diffed
// and saved — normalizing field names, filtering PII, merging multi-source
// data, or deriving computed columns. Registered per connector category so
// e.g. pfSense and OPNsense can share one firewall-rule transformer instead
// of duplicating logic per connector.
type Transformer interface {
	Transform(ctx context.Context, snap *connector.ServiceSnapshot) error
}

// TransformerFunc adapts a plain function to a Transformer.
type TransformerFunc func(ctx context.Context, snap *connector.ServiceSnapshot) error

// Transform calls fn.
func (fn TransformerFunc) Transform(ctx context.Context, snap *connector.ServiceSnapshot) error {
	return fn(ctx, snap)
}

var (
	transformersMu sync.RWMutex
	transformers   = map[string][]Transformer{}
)

// RegisterTransformer registers a Transformer to run, in registration order,
// on every snapshot fetched by a connector of the given category.
func RegisterTransformer(category string, t Transformer) {
	transformersMu.Lock()
	defer transformersMu.Unlock()
	transformers[category] = append(transformers[category], t)
}

// runTransformers applies every Transformer registered for category to snap,
// in order, stopping at the first error.
func runTransformers(ctx context.Context, category string, snap *connector.ServiceSnapshot) error {
	transformersMu.RLock()
	ts := transformers[category]
	transformersMu.RUnlock()
	for _, t := range ts {
		if err := t.Transform(ctx, snap); err != nil {
			return err
		}
	}
	return nil
}

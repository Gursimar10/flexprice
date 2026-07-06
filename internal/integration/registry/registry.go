package registry

import (
	"sync"

	"github.com/flexprice/flexprice/internal/types"
)

// Provider is a registry entry describing one integration provider.
type Provider struct {
	Type         types.SecretProvider
	Capabilities []Capability
	// NewInvoiceSyncHandler is nil when the provider does not support invoice sync.
	NewInvoiceSyncHandler func(HandlerDeps) InvoiceSyncHandler
}

var (
	mu        sync.RWMutex
	providers = map[types.SecretProvider]Provider{}
)

// Register adds or overwrites a provider entry keyed by its Type.
func Register(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	providers[p.Type] = p
}

// Get returns the provider entry for a type, if registered.
func Get(t types.SecretProvider) (Provider, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[t]
	return p, ok
}

// ProvidersWithCapability returns all registered providers declaring capability c.
func ProvidersWithCapability(c Capability) []Provider {
	mu.RLock()
	defer mu.RUnlock()
	var out []Provider
	for _, p := range providers {
		for _, cap := range p.Capabilities {
			if cap == c {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// resetForTest clears the registry. Test-only helper.
func resetForTest() {
	mu.Lock()
	defer mu.Unlock()
	providers = map[types.SecretProvider]Provider{}
}

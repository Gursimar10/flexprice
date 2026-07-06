package tabs

import (
	"github.com/flexprice/flexprice/internal/integration/registry"
	"github.com/flexprice/flexprice/internal/types"
)

// Register wires Tabs onto the generic integration pipeline. It is invoked once at startup
// via the integration providers package.
func Register() {
	registry.Register(registry.Provider{
		Type:         types.SecretProviderTabs,
		Capabilities: []registry.Capability{registry.CapabilityInvoiceSync},
		NewInvoiceSyncHandler: func(d registry.HandlerDeps) registry.InvoiceSyncHandler {
			return &tabsHandler{client: NewClient(d.Connection, d.EncryptionService, d.Logger), deps: d}
		},
	})
}

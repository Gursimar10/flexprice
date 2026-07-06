package tabs

import (
	"testing"

	"github.com/flexprice/flexprice/internal/integration/registry"
	"github.com/flexprice/flexprice/internal/types"
)

func TestRegister_AddsInvoiceSyncCapability(t *testing.T) {
	Register()

	p, ok := registry.Get(types.SecretProviderTabs)
	if !ok {
		t.Fatalf("expected tabs to be registered")
	}
	if p.NewInvoiceSyncHandler == nil {
		t.Fatalf("expected tabs to provide an invoice sync handler constructor")
	}

	found := false
	for _, c := range p.Capabilities {
		if c == registry.CapabilityInvoiceSync {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tabs to declare invoice_sync capability")
	}

	// The constructed handler must satisfy the registry contract.
	var _ registry.InvoiceSyncHandler = p.NewInvoiceSyncHandler(registry.HandlerDeps{})
}

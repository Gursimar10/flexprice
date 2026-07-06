package registry

import (
	"testing"

	"github.com/flexprice/flexprice/internal/types"
)

func TestRegisterAndGet(t *testing.T) {
	resetForTest()
	p := Provider{Type: types.SecretProviderStripe, Capabilities: []Capability{CapabilityInvoiceSync}}
	Register(p)

	got, ok := Get(types.SecretProviderStripe)
	if !ok || got.Type != types.SecretProviderStripe {
		t.Fatalf("expected to find registered provider, got ok=%v type=%v", ok, got.Type)
	}
	if _, ok := Get(types.SecretProviderPaddle); ok {
		t.Fatalf("did not expect unregistered provider to be found")
	}
}

func TestProvidersWithCapability(t *testing.T) {
	resetForTest()
	Register(Provider{Type: types.SecretProviderStripe, Capabilities: []Capability{CapabilityInvoiceSync}})
	Register(Provider{Type: types.SecretProviderHubSpot, Capabilities: []Capability{}})

	got := ProvidersWithCapability(CapabilityInvoiceSync)
	if len(got) != 1 || got[0].Type != types.SecretProviderStripe {
		t.Fatalf("expected only stripe to have invoice_sync capability, got %+v", got)
	}
}

func TestRegisterOverwritesByType(t *testing.T) {
	resetForTest()
	Register(Provider{Type: types.SecretProviderStripe, Capabilities: []Capability{}})
	Register(Provider{Type: types.SecretProviderStripe, Capabilities: []Capability{CapabilityInvoiceSync}})
	if len(ProvidersWithCapability(CapabilityInvoiceSync)) != 1 {
		t.Fatalf("expected re-registration to overwrite, not duplicate")
	}
}

package integration

import (
	"context"
	"testing"

	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/connection"
	"github.com/flexprice/flexprice/internal/domain/invoice"
	"github.com/flexprice/flexprice/internal/ee/service"
	"github.com/flexprice/flexprice/internal/integration/registry"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/temporal/models"
	"github.com/flexprice/flexprice/internal/testutil"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/stretchr/testify/require"
)

type fakeInvoiceSyncHandler struct {
	external string
	called   bool
}

func (f *fakeInvoiceSyncHandler) SyncInvoice(ctx context.Context, inv *invoice.Invoice) (registry.SyncResult, error) {
	f.called = true
	return registry.SyncResult{ExternalID: f.external, Metadata: map[string]any{"synced_via": "test"}}, nil
}

func testParams(ctx context.Context) service.ServiceParams {
	return service.ServiceParams{
		Logger: logger.NewNoopLogger(),
		Config: &config.Configuration{
			Secrets: config.SecretsConfig{EncryptionKey: "test-encryption-key-for-unit-tests-only"},
		},
		InvoiceRepo:                  testutil.NewInMemoryInvoiceStore(),
		ConnectionRepo:               testutil.NewInMemoryConnectionStore(),
		CustomerRepo:                 testutil.NewInMemoryCustomerStore(),
		EntityIntegrationMappingRepo: testutil.NewInMemoryEntityIntegrationMappingStore(),
	}
}

func seedTabsConnection(ctx context.Context, t *testing.T, connRepo connection.Repository) {
	t.Helper()
	require.NoError(t, connRepo.Create(ctx, &connection.Connection{
		ID:            "conn_tabs_test",
		ProviderType:  types.SecretProviderTabs,
		EnvironmentID: types.GetEnvironmentID(ctx),
		BaseModel: types.BaseModel{
			TenantID: types.GetTenantID(ctx),
			Status:   types.StatusPublished,
		},
	}))
}

func seedInvoice(ctx context.Context, t *testing.T, invoiceRepo invoice.Repository, id string) {
	t.Helper()
	require.NoError(t, invoiceRepo.Create(ctx, &invoice.Invoice{
		ID:            id,
		CustomerID:    "cust_001",
		EnvironmentID: types.GetEnvironmentID(ctx),
		BaseModel:     types.GetDefaultBaseModel(ctx),
	}))
}

func TestSyncInvoice_WritesMapping(t *testing.T) {
	ctx := testutil.SetupContext()
	params := testParams(ctx)

	handler := &fakeInvoiceSyncHandler{external: "tabs_inv_123"}
	registry.Register(registry.Provider{
		Type:         types.SecretProviderTabs,
		Capabilities: []registry.Capability{registry.CapabilityInvoiceSync},
		NewInvoiceSyncHandler: func(d registry.HandlerDeps) registry.InvoiceSyncHandler {
			return handler
		},
	})

	seedTabsConnection(ctx, t, params.ConnectionRepo)
	seedInvoice(ctx, t, params.InvoiceRepo, "inv_1")

	act := NewIntegrationSyncActivities(params, logger.NewNoopLogger())
	err := act.SyncInvoice(ctx, models.IntegrationInvoiceSyncWorkflowInput{
		Provider:      types.SecretProviderTabs,
		InvoiceID:     "inv_1",
		TenantID:      types.GetTenantID(ctx),
		EnvironmentID: types.GetEnvironmentID(ctx),
	})
	require.NoError(t, err)
	require.True(t, handler.called, "handler should have been invoked")

	filter := types.NewNoLimitEntityIntegrationMappingFilter()
	filter.EntityID = "inv_1"
	filter.EntityType = types.IntegrationEntityTypeInvoice
	filter.ProviderTypes = []string{string(types.SecretProviderTabs)}
	count, err := params.EntityIntegrationMappingRepo.Count(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, count, "expected exactly one invoice→tabs mapping to be written")

	mappings, err := params.EntityIntegrationMappingRepo.List(ctx, filter)
	require.NoError(t, err)
	require.Len(t, mappings, 1)
	require.Equal(t, "tabs_inv_123", mappings[0].ProviderEntityID)
}

func TestSyncInvoice_UnregisteredProvider_Errors(t *testing.T) {
	ctx := testutil.SetupContext()
	params := testParams(ctx)

	act := NewIntegrationSyncActivities(params, logger.NewNoopLogger())
	err := act.SyncInvoice(ctx, models.IntegrationInvoiceSyncWorkflowInput{
		Provider:      types.SecretProvider("not_registered"),
		InvoiceID:     "inv_x",
		TenantID:      types.GetTenantID(ctx),
		EnvironmentID: types.GetEnvironmentID(ctx),
	})
	require.Error(t, err)
}

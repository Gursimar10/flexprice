package integration

import (
	"context"

	"github.com/flexprice/flexprice/internal/domain/entityintegrationmapping"
	"github.com/flexprice/flexprice/internal/ee/service"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/integration/registry"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/security"
	"github.com/flexprice/flexprice/internal/temporal/models"
	"github.com/flexprice/flexprice/internal/types"
)

// IntegrationSyncActivities is the single generic activity set for the integration pipeline.
// It resolves provider-specific logic from the registry, so new providers need no new activity.
type IntegrationSyncActivities struct {
	params service.ServiceParams
	logger *logger.Logger
}

// NewIntegrationSyncActivities creates the generic integration sync activities handler.
func NewIntegrationSyncActivities(params service.ServiceParams, logger *logger.Logger) *IntegrationSyncActivities {
	return &IntegrationSyncActivities{params: params, logger: logger}
}

// SyncInvoice resolves the provider handler from the registry, syncs the invoice, and
// records the entity-integration mapping generically so the idempotency guard works.
func (a *IntegrationSyncActivities) SyncInvoice(ctx context.Context, input models.IntegrationInvoiceSyncWorkflowInput) error {
	ctx = types.SetTenantID(ctx, input.TenantID)
	ctx = types.SetEnvironmentID(ctx, input.EnvironmentID)

	a.logger.Info(ctx, "integration_pipeline: syncing invoice",
		"provider", input.Provider, "invoice_id", input.InvoiceID)

	prov, ok := registry.Get(input.Provider)
	if !ok || prov.NewInvoiceSyncHandler == nil {
		return ierr.NewError("no invoice sync handler registered").
			WithHint("Provider is not registered for invoice sync").
			WithReportableDetails(map[string]interface{}{"provider": input.Provider}).
			Mark(ierr.ErrValidation)
	}

	inv, err := a.params.InvoiceRepo.Get(ctx, input.InvoiceID)
	if err != nil {
		return err
	}

	conn, err := a.params.ConnectionRepo.GetByProvider(ctx, input.Provider)
	if err != nil {
		return err
	}

	encryptionSvc, err := security.NewEncryptionService(a.params.Config, a.logger)
	if err != nil {
		return err
	}

	handler := prov.NewInvoiceSyncHandler(registry.HandlerDeps{
		Logger:                       a.logger,
		Config:                       a.params.Config,
		Connection:                   conn,
		EncryptionService:            encryptionSvc,
		CustomerRepo:                 a.params.CustomerRepo,
		SubscriptionRepo:             a.params.SubRepo,
		PlanRepo:                     a.params.PlanRepo,
		EntityIntegrationMappingRepo: a.params.EntityIntegrationMappingRepo,
	})

	res, err := handler.SyncInvoice(ctx, inv)
	if err != nil {
		a.logger.Error(ctx, "integration_pipeline: invoice sync failed",
			"provider", input.Provider, "invoice_id", input.InvoiceID, "error", err)
		return err
	}

	mapping := &entityintegrationmapping.EntityIntegrationMapping{
		ID:               types.GenerateUUIDWithPrefix(types.UUID_PREFIX_ENTITY_INTEGRATION_MAPPING),
		EntityType:       types.IntegrationEntityTypeInvoice,
		EntityID:         input.InvoiceID,
		ProviderType:     string(input.Provider),
		ProviderEntityID: res.ExternalID,
		Metadata:         res.Metadata,
		EnvironmentID:    input.EnvironmentID,
		BaseModel:        types.GetDefaultBaseModel(ctx),
	}
	if err := a.params.EntityIntegrationMappingRepo.Create(ctx, mapping); err != nil {
		return err
	}

	a.logger.Info(ctx, "integration_pipeline: invoice synced",
		"provider", input.Provider, "invoice_id", input.InvoiceID, "external_id", res.ExternalID)
	return nil
}

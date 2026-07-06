package tabs

import (
	"context"

	"github.com/flexprice/flexprice/internal/domain/customer"
	"github.com/flexprice/flexprice/internal/domain/entityintegrationmapping"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/samber/lo"
)

// ensureCustomer guarantees a flexprice-customer -> tabs-customer mapping exists, creating the
// customer in Tabs (async job) if needed, and returns the Tabs customer id. It is idempotent:
// if a mapping already exists it is returned without calling Tabs.
func (h *tabsHandler) ensureCustomer(ctx context.Context, cust *customer.Customer, currency string) (string, error) {
	filter := types.NewNoLimitEntityIntegrationMappingFilter()
	filter.EntityID = cust.ID
	filter.EntityType = types.IntegrationEntityTypeCustomer
	filter.ProviderTypes = []string{string(types.SecretProviderTabs)}
	filter.Status = lo.ToPtr(types.StatusPublished)

	existing, err := h.deps.EntityIntegrationMappingRepo.List(ctx, filter)
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return existing[0].ProviderEntityID, nil
	}

	jobID, err := h.client.CreateCustomer(ctx, CreateCustomerRequest{
		Name:                       cust.Name,
		PrimaryBillingContactEmail: cust.Email,
		DefaultCurrency:            currency,
	})
	if err != nil {
		return "", err
	}

	payload, err := h.client.WaitForJob(ctx, jobID)
	if err != nil {
		return "", err
	}

	tabsCustomerID, _ := payload.Results.Data["customerId"].(string)
	if tabsCustomerID == "" {
		return "", ierr.NewError("tabs customer id missing from job result").
			WithHint("Tabs create-customer job succeeded but returned no customerId").
			WithReportableDetails(map[string]interface{}{"job_id": jobID}).
			Mark(ierr.ErrSystem)
	}

	mapping := &entityintegrationmapping.EntityIntegrationMapping{
		ID:               types.GenerateUUIDWithPrefix(types.UUID_PREFIX_ENTITY_INTEGRATION_MAPPING),
		EntityID:         cust.ID,
		EntityType:       types.IntegrationEntityTypeCustomer,
		ProviderType:     string(types.SecretProviderTabs),
		ProviderEntityID: tabsCustomerID,
		EnvironmentID:    types.GetEnvironmentID(ctx),
		BaseModel:        types.GetDefaultBaseModel(ctx),
	}
	if err := h.deps.EntityIntegrationMappingRepo.Create(ctx, mapping); err != nil {
		return "", err
	}

	h.deps.Logger.Info(ctx, "tabs: customer created and mapped",
		"flexprice_customer_id", cust.ID, "tabs_customer_id", tabsCustomerID)
	return tabsCustomerID, nil
}

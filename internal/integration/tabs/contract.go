package tabs

import (
	"context"
	"fmt"

	"github.com/flexprice/flexprice/internal/domain/entityintegrationmapping"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/samber/lo"
)

// ensureContract guarantees a flexprice-subscription -> tabs-contract mapping exists, creating
// the contract in Tabs (synchronous) if needed, and returns the Tabs contract id. It is
// idempotent: if a mapping already exists it is returned without calling Tabs.
func (h *tabsHandler) ensureContract(ctx context.Context, subscriptionID, tabsCustomerID string) (string, error) {
	filter := types.NewNoLimitEntityIntegrationMappingFilter()
	filter.EntityID = subscriptionID
	filter.EntityType = types.IntegrationEntityTypeSubscription
	filter.ProviderTypes = []string{string(types.SecretProviderTabs)}
	filter.Status = lo.ToPtr(types.StatusPublished)

	existing, err := h.deps.EntityIntegrationMappingRepo.List(ctx, filter)
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return existing[0].ProviderEntityID, nil
	}

	contractID, err := h.client.CreateContract(ctx, CreateContractRequest{
		Name:       h.contractName(ctx, subscriptionID),
		CustomerID: tabsCustomerID,
	})
	if err != nil {
		return "", err
	}

	// The contract is created in NEW status; mark it PROCESSED before persisting the mapping so
	// a mapping only ever records a fully-processed contract.
	if err := h.client.MarkContractProcessed(ctx, contractID); err != nil {
		return "", err
	}

	mapping := &entityintegrationmapping.EntityIntegrationMapping{
		ID:               types.GenerateUUIDWithPrefix(types.UUID_PREFIX_ENTITY_INTEGRATION_MAPPING),
		EntityID:         subscriptionID,
		EntityType:       types.IntegrationEntityTypeSubscription,
		ProviderType:     string(types.SecretProviderTabs),
		ProviderEntityID: contractID,
		EnvironmentID:    types.GetEnvironmentID(ctx),
		BaseModel:        types.GetDefaultBaseModel(ctx),
	}
	if err := h.deps.EntityIntegrationMappingRepo.Create(ctx, mapping); err != nil {
		return "", err
	}

	h.deps.Logger.Info(ctx, "tabs: contract created and mapped",
		"flexprice_subscription_id", subscriptionID, "tabs_contract_id", contractID)
	return contractID, nil
}

// contractName derives the Tabs contract name from the subscription's plan name, falling back
// to a subscription-id based name if the subscription or plan cannot be resolved.
func (h *tabsHandler) contractName(ctx context.Context, subscriptionID string) string {
	fallback := fmt.Sprintf("Flexprice Subscription %s", subscriptionID)

	sub, err := h.deps.SubscriptionRepo.Get(ctx, subscriptionID)
	if err != nil || sub == nil || sub.PlanID == "" {
		return fallback
	}
	pl, err := h.deps.PlanRepo.Get(ctx, sub.PlanID)
	if err != nil || pl == nil || pl.Name == "" {
		return fallback
	}
	return pl.Name
}

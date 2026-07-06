package tabs

import (
	"context"

	"github.com/flexprice/flexprice/internal/domain/invoice"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/integration/registry"
)

// tabsHandler implements registry.InvoiceSyncHandler for the Tabs provider.
type tabsHandler struct {
	client *Client
	deps   registry.HandlerDeps
}

// SyncInvoice pushes a FlexPrice invoice to Tabs.
//
// Step 1 (implemented): ensure the invoice's customer exists in Tabs and is mapped.
// Step 2 (implemented): if the invoice belongs to a subscription, ensure that subscription is
// mapped to a Tabs contract.
// Step 3 (TODO tabs-api): create the invoice in Tabs using the mapped customer/contract ids and
// return the external invoice id. Until step 3 is implemented this returns an error after the
// customer and contract mappings are established (both idempotent, so retries are safe).
func (h *tabsHandler) SyncInvoice(ctx context.Context, inv *invoice.Invoice) (registry.SyncResult, error) {
	cust, err := h.deps.CustomerRepo.Get(ctx, inv.CustomerID)
	if err != nil {
		return registry.SyncResult{}, err
	}

	tabsCustomerID, err := h.ensureCustomer(ctx, cust, inv.Currency)
	if err != nil {
		return registry.SyncResult{}, err
	}

	if inv.SubscriptionID != nil && *inv.SubscriptionID != "" {
		tabsContractID, cErr := h.ensureContract(ctx, *inv.SubscriptionID, tabsCustomerID)
		if cErr != nil {
			return registry.SyncResult{}, cErr
		}
		h.deps.Logger.Info(ctx, "tabs: contract ensured for invoice",
			"invoice_id", inv.ID,
			"flexprice_subscription_id", *inv.SubscriptionID,
			"tabs_contract_id", tabsContractID)
	}

	h.deps.Logger.Info(ctx, "tabs: customer ensured for invoice",
		"invoice_id", inv.ID,
		"flexprice_customer_id", cust.ID,
		"tabs_customer_id", tabsCustomerID)

	// TODO(tabs-api): create the invoice in Tabs using tabsCustomerID (+ contract), then return
	// its external id.
	return registry.SyncResult{}, ierr.NewError("tabs invoice creation not yet implemented").
		WithHint("Tabs customer/contract mappings are ready; invoice push pending API spec").
		Mark(ierr.ErrSystem)
}

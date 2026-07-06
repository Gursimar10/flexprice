package registry

import (
	"context"

	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/connection"
	"github.com/flexprice/flexprice/internal/domain/customer"
	"github.com/flexprice/flexprice/internal/domain/entityintegrationmapping"
	"github.com/flexprice/flexprice/internal/domain/invoice"
	"github.com/flexprice/flexprice/internal/domain/plan"
	"github.com/flexprice/flexprice/internal/domain/subscription"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/security"
)

// Capability names an operation a provider can perform on the generic pipeline.
type Capability string

const (
	CapabilityInvoiceSync Capability = "invoice_sync"
	// Future: CapabilityCustomerSync, CapabilityInvoicePull, CapabilityMarkPaid, ...
)

// SyncResult is what a handler returns so the pipeline can persist the mapping generically.
type SyncResult struct {
	ExternalID string
	Metadata   map[string]any
}

// HandlerDeps are the shared dependencies handed to a handler constructor at runtime.
// Extend this struct (one place) as future handlers need more.
type HandlerDeps struct {
	Logger     *logger.Logger
	Config     *config.Configuration
	Connection *connection.Connection
	// EncryptionService lets a handler decrypt its own credentials from Connection.
	EncryptionService security.EncryptionService
	// CustomerRepo / SubscriptionRepo / PlanRepo let a handler resolve entities referenced
	// by an invoice (e.g. to name a provider contract after the subscription's plan).
	CustomerRepo     customer.Repository
	SubscriptionRepo subscription.Repository
	PlanRepo         plan.Repository
	// EntityIntegrationMappingRepo persists entity mappings (flexprice entity -> provider entity).
	EntityIntegrationMappingRepo entityintegrationmapping.Repository
}

// InvoiceSyncHandler pushes a FlexPrice invoice to the provider and returns the external id.
type InvoiceSyncHandler interface {
	SyncInvoice(ctx context.Context, inv *invoice.Invoice) (SyncResult, error)
}

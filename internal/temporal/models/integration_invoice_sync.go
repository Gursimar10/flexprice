package models

import (
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/types"
)

// IntegrationInvoiceSyncWorkflowInput is the provider-agnostic input for the generic
// invoice sync workflow. Provider selects which registered handler runs.
type IntegrationInvoiceSyncWorkflowInput struct {
	Provider      types.SecretProvider `json:"provider"`
	InvoiceID     string               `json:"invoice_id"`
	TenantID      string               `json:"tenant_id"`
	EnvironmentID string               `json:"environment_id"`
}

// Validate validates the workflow input.
func (input *IntegrationInvoiceSyncWorkflowInput) Validate() error {
	if input.Provider == "" {
		return ierr.NewError("provider is required").WithHint("Provider must not be empty").Mark(ierr.ErrValidation)
	}
	if input.InvoiceID == "" {
		return ierr.NewError("invoice_id is required").WithHint("InvoiceID must not be empty").Mark(ierr.ErrValidation)
	}
	if input.TenantID == "" {
		return ierr.NewError("tenant_id is required").WithHint("TenantID must not be empty").Mark(ierr.ErrValidation)
	}
	if input.EnvironmentID == "" {
		return ierr.NewError("environment_id is required").WithHint("EnvironmentID must not be empty").Mark(ierr.ErrValidation)
	}
	return nil
}

package workflows

import (
	"time"

	"github.com/flexprice/flexprice/internal/temporal/models"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// WorkflowIntegrationInvoiceSync must equal the workflow function name.
	WorkflowIntegrationInvoiceSync = "IntegrationInvoiceSyncWorkflow"
	// ActivityIntegrationSyncInvoice must equal the registered activity method name.
	ActivityIntegrationSyncInvoice = "SyncInvoice"
)

// IntegrationInvoiceSyncWorkflow is the single provider-agnostic invoice-sync workflow.
// The provider-specific logic is resolved from the integration registry inside the activity,
// so new providers plug in without defining a new workflow.
func IntegrationInvoiceSyncWorkflow(ctx workflow.Context, input models.IntegrationInvoiceSyncWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting integration invoice sync workflow",
		"provider", input.Provider, "invoice_id", input.InvoiceID,
		"tenant_id", input.TenantID, "environment_id", input.EnvironmentID)

	if err := input.Validate(); err != nil {
		logger.Error("Invalid workflow input", "error", err)
		return err
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})

	// Match existing invoice-sync workflows: let the invoice DB txn commit before fetching.
	if err := workflow.Sleep(ctx, 5*time.Second); err != nil {
		logger.Error("Sleep was interrupted", "error", err)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, ActivityIntegrationSyncInvoice, input).Get(ctx, nil); err != nil {
		logger.Error("Failed to sync invoice", "error", err, "provider", input.Provider, "invoice_id", input.InvoiceID)
		return err
	}

	logger.Info("Successfully completed integration invoice sync workflow",
		"provider", input.Provider, "invoice_id", input.InvoiceID)
	return nil
}

package events

import (
	"context"
	"testing"

	"github.com/flexprice/flexprice/internal/domain/connection"
	"github.com/flexprice/flexprice/internal/domain/entityintegrationmapping"
	"github.com/flexprice/flexprice/internal/integration/registry"
	tmodels "github.com/flexprice/flexprice/internal/temporal/models"
	temporalservice "github.com/flexprice/flexprice/internal/temporal/service"
	"github.com/flexprice/flexprice/internal/types"
)

// fakeConnRepo overrides only GetByProvider; the embedded interface is nil so any other
// method call would panic, which keeps the fake honest about what the code under test uses.
type fakeConnRepo struct {
	connection.Repository
	conn *connection.Connection
	err  error
}

func (f fakeConnRepo) GetByProvider(ctx context.Context, p types.SecretProvider) (*connection.Connection, error) {
	return f.conn, f.err
}

type fakeEIMRepo struct {
	entityintegrationmapping.Repository
	count int
}

func (f fakeEIMRepo) Count(ctx context.Context, _ *types.EntityIntegrationMappingFilter) (int, error) {
	return f.count, nil
}

type fakeRun struct{}

func (fakeRun) GetID() string                              { return "wf_1" }
func (fakeRun) GetRunID() string                           { return "run_1" }
func (fakeRun) Get(_ context.Context, _ interface{}) error { return nil }

type fakeTemporal struct {
	temporalservice.TemporalService
	started int
	lastWF  types.TemporalWorkflowType
}

func (f *fakeTemporal) ExecuteWorkflow(_ context.Context, wf types.TemporalWorkflowType, _ interface{}) (tmodels.WorkflowRun, error) {
	f.started++
	f.lastWF = wf
	return fakeRun{}, nil
}

func tabsProvider() registry.Provider {
	return registry.Provider{Type: types.SecretProviderTabs, Capabilities: []registry.Capability{registry.CapabilityInvoiceSync}}
}

func connOutboundEnabled() *connection.Connection {
	return &connection.Connection{
		ProviderType: types.SecretProviderTabs,
		SyncConfig:   &types.SyncConfig{Invoice: &types.EntitySyncConfig{Outbound: true}},
	}
}

func TestTriggerGenericInvoiceSync_NoConnection_Skips(t *testing.T) {
	tp := &fakeTemporal{}
	err := triggerGenericInvoiceSync(context.Background(), fakeConnRepo{conn: nil}, fakeEIMRepo{}, tp, testLogger(), tabsProvider(),
		invoiceVendorSyncInput{TenantID: "t", EnvironmentID: "e", InvoiceID: "inv_1"})
	if err != nil || tp.started != 0 {
		t.Fatalf("expected skip with no workflow, err=%v started=%d", err, tp.started)
	}
}

func TestTriggerGenericInvoiceSync_OutboundDisabled_Skips(t *testing.T) {
	tp := &fakeTemporal{}
	conn := &connection.Connection{ProviderType: types.SecretProviderTabs} // no sync config -> outbound disabled
	err := triggerGenericInvoiceSync(context.Background(), fakeConnRepo{conn: conn}, fakeEIMRepo{}, tp, testLogger(), tabsProvider(),
		invoiceVendorSyncInput{TenantID: "t", EnvironmentID: "e", InvoiceID: "inv_1"})
	if err != nil || tp.started != 0 {
		t.Fatalf("expected skip when outbound disabled, err=%v started=%d", err, tp.started)
	}
}

func TestTriggerGenericInvoiceSync_AlreadySynced_Skips(t *testing.T) {
	tp := &fakeTemporal{}
	err := triggerGenericInvoiceSync(context.Background(), fakeConnRepo{conn: connOutboundEnabled()}, fakeEIMRepo{count: 1}, tp, testLogger(), tabsProvider(),
		invoiceVendorSyncInput{TenantID: "t", EnvironmentID: "e", InvoiceID: "inv_1"})
	if err != nil || tp.started != 0 {
		t.Fatalf("expected skip when already synced, err=%v started=%d", err, tp.started)
	}
}

func TestTriggerGenericInvoiceSync_StartsWorkflow(t *testing.T) {
	tp := &fakeTemporal{}
	err := triggerGenericInvoiceSync(context.Background(), fakeConnRepo{conn: connOutboundEnabled()}, fakeEIMRepo{count: 0}, tp, testLogger(), tabsProvider(),
		invoiceVendorSyncInput{TenantID: "t", EnvironmentID: "e", InvoiceID: "inv_1"})
	if err != nil || tp.started != 1 || tp.lastWF != types.TemporalIntegrationInvoiceSyncWorkflow {
		t.Fatalf("expected one generic workflow, err=%v started=%d wf=%v", err, tp.started, tp.lastWF)
	}
}

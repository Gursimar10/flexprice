package tabs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flexprice/flexprice/internal/domain/plan"
	"github.com/flexprice/flexprice/internal/domain/subscription"
	"github.com/flexprice/flexprice/internal/integration/registry"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/testutil"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/stretchr/testify/require"
)

func TestEnsureContract_CreatesAndMaps(t *testing.T) {
	ctx := testutil.SetupContext()
	enc := testEncryption(t)

	var (
		body        CreateContractRequest
		createCalls int
		actionBody  ContractActionRequest
		actionPath  string
		actionCalls int
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/contracts", func(w http.ResponseWriter, r *http.Request) {
		createCalls++
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"payload":{"id":"contract_1","status":"NEW","customerId":"tabs_cust_1"}}`))
	})
	// Subtree handler for POST /v3/contracts/{id}/actions.
	mux.HandleFunc("/v3/contracts/", func(w http.ResponseWriter, r *http.Request) {
		actionCalls++
		actionPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&actionBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"payload":"Contract status updated successfully to PROCESSED"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(testTabsConnection(t, enc, "k"), enc, logger.NewNoopLogger())
	client.baseURL = srv.URL

	// Seed a plan + subscription so the contract is named after the plan.
	planStore := testutil.NewInMemoryPlanStore()
	require.NoError(t, planStore.Create(ctx, &plan.Plan{
		ID: "plan_1", Name: "Pro Plan", BaseModel: types.GetDefaultBaseModel(ctx),
	}))
	subStore := testutil.NewInMemorySubscriptionStore()
	require.NoError(t, subStore.Create(ctx, &subscription.Subscription{
		ID: "sub_1", PlanID: "plan_1", BaseModel: types.GetDefaultBaseModel(ctx),
	}))

	eimRepo := testutil.NewInMemoryEntityIntegrationMappingStore()
	h := &tabsHandler{client: client, deps: registry.HandlerDeps{
		Logger:                       logger.NewNoopLogger(),
		SubscriptionRepo:             subStore,
		PlanRepo:                     planStore,
		EntityIntegrationMappingRepo: eimRepo,
	}}

	id, err := h.ensureContract(ctx, "sub_1", "tabs_cust_1")
	require.NoError(t, err)
	require.Equal(t, "contract_1", id)
	require.Equal(t, "tabs_cust_1", body.CustomerID)
	require.Equal(t, "Pro Plan", body.Name, "contract must be named after the subscription's plan")
	require.Equal(t, 1, actionCalls, "contract must be marked as processed exactly once")
	require.Equal(t, "MARK_AS_PROCESSED", actionBody.Action)
	require.Equal(t, "/v3/contracts/contract_1/actions", actionPath)

	filter := types.NewNoLimitEntityIntegrationMappingFilter()
	filter.EntityID = "sub_1"
	filter.EntityType = types.IntegrationEntityTypeSubscription
	filter.ProviderTypes = []string{string(types.SecretProviderTabs)}
	count, err := eimRepo.Count(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Idempotency: existing mapping returned without re-creating.
	id2, err := h.ensureContract(ctx, "sub_1", "tabs_cust_1")
	require.NoError(t, err)
	require.Equal(t, "contract_1", id2)
	require.Equal(t, 1, createCalls, "second ensure must not re-create the contract")
	require.Equal(t, 1, actionCalls, "second ensure must not re-process the contract")
}

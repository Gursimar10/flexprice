package tabs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/connection"
	"github.com/flexprice/flexprice/internal/domain/customer"
	"github.com/flexprice/flexprice/internal/integration/registry"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/security"
	"github.com/flexprice/flexprice/internal/testutil"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/stretchr/testify/require"
)

func testEncryption(t *testing.T) security.EncryptionService {
	t.Helper()
	enc, err := security.NewEncryptionService(&config.Configuration{
		Secrets: config.SecretsConfig{EncryptionKey: "test-encryption-key-for-unit-tests-only"},
	}, logger.NewNoopLogger())
	require.NoError(t, err)
	return enc
}

func testTabsConnection(t *testing.T, enc security.EncryptionService, plainKey string) *connection.Connection {
	t.Helper()
	encKey, err := enc.Encrypt(plainKey)
	require.NoError(t, err)
	return &connection.Connection{
		ProviderType: types.SecretProviderTabs,
		EncryptedSecretData: types.ConnectionMetadata{
			Tabs: &types.TabsConnectionMetadata{APIKey: encKey},
		},
	}
}

func TestEnsureCustomer_CreatesPollsAndMaps(t *testing.T) {
	ctx := testutil.SetupContext()
	enc := testEncryption(t)

	var (
		authSeen    string
		createBody  CreateCustomerRequest
		createCalls int
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/customers", func(w http.ResponseWriter, r *http.Request) {
		createCalls++
		authSeen = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&createBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"payload":{"jobId":"job_1","statusEndpoint":"/v3/jobs/job_1"}}`))
	})
	mux.HandleFunc("/v3/jobs/job_1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"payload":{"id":"job_1","status":"SUCCESS","type":"CREATE_CUSTOMER","results":{"status":"SUCCESS","data":{"customerId":"tabs_cust_1"}}}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(testTabsConnection(t, enc, "secret-api-key"), enc, logger.NewNoopLogger())
	client.baseURL = srv.URL

	eimRepo := testutil.NewInMemoryEntityIntegrationMappingStore()
	h := &tabsHandler{client: client, deps: registry.HandlerDeps{
		Logger:                       logger.NewNoopLogger(),
		EntityIntegrationMappingRepo: eimRepo,
	}}

	cust := &customer.Customer{ID: "cust_1", Name: "Acme Inc", Email: "billing@acme.test"}

	id, err := h.ensureCustomer(ctx, cust, "USD")
	require.NoError(t, err)
	require.Equal(t, "tabs_cust_1", id)
	require.Equal(t, "secret-api-key", authSeen, "decrypted api key must be sent as Authorization")
	require.Equal(t, "Acme Inc", createBody.Name)
	require.Equal(t, "billing@acme.test", createBody.PrimaryBillingContactEmail)
	require.Equal(t, "USD", createBody.DefaultCurrency)

	filter := types.NewNoLimitEntityIntegrationMappingFilter()
	filter.EntityID = "cust_1"
	filter.EntityType = types.IntegrationEntityTypeCustomer
	filter.ProviderTypes = []string{string(types.SecretProviderTabs)}
	count, err := eimRepo.Count(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Idempotency: an existing mapping is returned without calling Tabs again.
	id2, err := h.ensureCustomer(ctx, cust, "USD")
	require.NoError(t, err)
	require.Equal(t, "tabs_cust_1", id2)
	require.Equal(t, 1, createCalls, "second ensure must not re-create the customer in Tabs")
}

func TestEnsureCustomer_JobFailed_Errors(t *testing.T) {
	ctx := testutil.SetupContext()
	enc := testEncryption(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v3/customers", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"payload":{"jobId":"job_2"}}`))
	})
	mux.HandleFunc("/v3/jobs/job_2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"payload":{"id":"job_2","status":"FAILED"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(testTabsConnection(t, enc, "k"), enc, logger.NewNoopLogger())
	client.baseURL = srv.URL

	h := &tabsHandler{client: client, deps: registry.HandlerDeps{
		Logger:                       logger.NewNoopLogger(),
		EntityIntegrationMappingRepo: testutil.NewInMemoryEntityIntegrationMappingStore(),
	}}

	_, err := h.ensureCustomer(ctx, &customer.Customer{ID: "cust_2", Name: "N", Email: "e@e.test"}, "USD")
	require.Error(t, err)
}

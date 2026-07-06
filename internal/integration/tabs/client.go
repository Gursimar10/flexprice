package tabs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flexprice/flexprice/internal/domain/connection"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/security"
)

// defaultBaseURL is the Tabs production integrators API base URL.
const defaultBaseURL = "https://integrators.prod.api.tabsplatform.com"

// Client talks to the Tabs API. The API key is read (decrypted) from the tenant's connection.
type Client struct {
	http       *http.Client
	baseURL    string
	conn       *connection.Connection
	encryption security.EncryptionService
	logger     *logger.Logger
}

// NewClient builds a Tabs API client bound to a tenant's connection (which carries the api key).
func NewClient(conn *connection.Connection, enc security.EncryptionService, log *logger.Logger) *Client {
	return &Client{
		http:       &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
		conn:       conn,
		encryption: enc,
		logger:     log,
	}
}

// apiKey returns the decrypted Tabs API key from the connection.
func (c *Client) apiKey() (string, error) {
	if c.conn == nil || c.conn.EncryptedSecretData.Tabs == nil || c.conn.EncryptedSecretData.Tabs.APIKey == "" {
		return "", ierr.NewError("tabs api key not configured").
			WithHint("Tabs connection is missing an api_key").
			Mark(ierr.ErrValidation)
	}
	return c.encryption.Decrypt(c.conn.EncryptedSecretData.Tabs.APIKey)
}

// doRequest performs an authenticated JSON request against the Tabs API.
func (c *Client) doRequest(ctx context.Context, method, path string, body any, out any) error {
	apiKey, err := c.apiKey()
	if err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		b, mErr := json.Marshal(body)
		if mErr != nil {
			return mErr
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ierr.NewError("tabs api request failed").
			WithHintf("Tabs API returned status %d", resp.StatusCode).
			WithReportableDetails(map[string]interface{}{
				"status": resp.StatusCode,
				"path":   path,
				"body":   string(respBytes),
			}).
			Mark(ierr.ErrSystem)
	}

	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return err
		}
	}
	return nil
}

// CreateCustomer starts an async customer-creation job and returns its job id.
func (c *Client) CreateCustomer(ctx context.Context, req CreateCustomerRequest) (string, error) {
	var resp createCustomerResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v3/customers", req, &resp); err != nil {
		return "", err
	}
	if resp.Payload.JobID == "" {
		return "", ierr.NewError("tabs did not return a job id").
			WithHint("Tabs create-customer response had no jobId").
			Mark(ierr.ErrSystem)
	}
	return resp.Payload.JobID, nil
}

// CreateContract creates a contract in Tabs and returns its id. Contract creation is
// synchronous (shouldProcess=true), so the id is available in the response.
func (c *Client) CreateContract(ctx context.Context, req CreateContractRequest) (string, error) {
	var resp createContractResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v3/contracts", req, &resp); err != nil {
		return "", err
	}
	if resp.Payload.ID == "" {
		return "", ierr.NewError("tabs did not return a contract id").
			WithHint("Tabs create-contract response had no id").
			Mark(ierr.ErrSystem)
	}
	return resp.Payload.ID, nil
}

// MarkContractProcessed transitions a Tabs contract from NEW to PROCESSED. The response body is
// a plain success message, so only the HTTP status matters.
func (c *Client) MarkContractProcessed(ctx context.Context, contractID string) error {
	return c.doRequest(ctx, http.MethodPost, "/v3/contracts/"+contractID+"/actions",
		ContractActionRequest{Action: ContractActionMarkAsProcessed}, nil)
}

// GetJob returns the current state of a Tabs async job.
func (c *Client) GetJob(ctx context.Context, jobID string) (*JobPayload, error) {
	var resp jobResponse
	if err := c.doRequest(ctx, http.MethodGet, "/v3/jobs/"+jobID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Payload, nil
}

// jobPollAttempts / jobPollInterval bound how long WaitForJob polls a Tabs job.
// The activity's StartToCloseTimeout (5m) is the outer bound.
var (
	jobPollAttempts = 30
	jobPollInterval = 2 * time.Second
)

// WaitForJob polls a Tabs job until it succeeds, fails, or times out.
func (c *Client) WaitForJob(ctx context.Context, jobID string) (*JobPayload, error) {
	for attempt := 0; attempt < jobPollAttempts; attempt++ {
		payload, err := c.GetJob(ctx, jobID)
		if err != nil {
			return nil, err
		}
		switch strings.ToUpper(payload.Status) {
		case "SUCCESS":
			return payload, nil
		case "FAILED", "ERROR":
			return nil, ierr.NewError("tabs job failed").
				WithHintf("Tabs job %s reported status %s", jobID, payload.Status).
				WithReportableDetails(map[string]interface{}{"job_id": jobID, "status": payload.Status}).
				Mark(ierr.ErrSystem)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(jobPollInterval):
		}
	}
	return nil, ierr.NewError("tabs job timed out").
		WithHintf("Tabs job %s did not complete within the polling window", jobID).
		WithReportableDetails(map[string]interface{}{"job_id": jobID}).
		Mark(ierr.ErrSystem)
}

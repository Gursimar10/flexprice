package tabs

// CreateCustomerRequest is the body for POST /v3/customers.
type CreateCustomerRequest struct {
	Name                       string `json:"name"`
	PrimaryBillingContactEmail string `json:"primaryBillingContactEmail"`
	DefaultCurrency            string `json:"defaultCurrency"`
}

// createCustomerResponse is the envelope returned by POST /v3/customers.
// Tabs runs customer creation as an async job; the real id is fetched from the job.
type createCustomerResponse struct {
	Payload struct {
		JobID          string `json:"jobId"`
		StatusEndpoint string `json:"statusEndpoint"`
		Message        string `json:"message"`
	} `json:"payload"`
	Success bool `json:"success"`
}

// CreateContractRequest is the body for POST /v3/contracts.
type CreateContractRequest struct {
	Name       string `json:"name"`
	CustomerID string `json:"customerId"` // Tabs customer id
}

// createContractResponse is the envelope returned by POST /v3/contracts.
// Contract creation is synchronous: the contract id is in the payload directly.
type createContractResponse struct {
	Payload struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		CustomerID string `json:"customerId"`
	} `json:"payload"`
	Success bool `json:"success"`
}

// ContractActionRequest is the body for POST /v3/contracts/{id}/actions.
type ContractActionRequest struct {
	Action string `json:"action"`
}

// ContractActionMarkAsProcessed transitions a contract from NEW to PROCESSED.
const ContractActionMarkAsProcessed = "MARK_AS_PROCESSED"

// JobPayload is the job state returned by GET /v3/jobs/{jobId}.
type JobPayload struct {
	ID      string `json:"id"`
	Status  string `json:"status"` // e.g. SUCCESS, FAILED, IN_PROGRESS
	Type    string `json:"type"`
	Results struct {
		Data   map[string]any `json:"data"`
		Status string         `json:"status"`
	} `json:"results"`
}

// jobResponse is the envelope returned by GET /v3/jobs/{jobId}.
type jobResponse struct {
	Payload JobPayload `json:"payload"`
	Success bool       `json:"success"`
}

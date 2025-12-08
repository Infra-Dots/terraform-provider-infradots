package internal

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRoundTripper implements http.RoundTripper for testing purposes
type MockRoundTripper struct{}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a mocked response based on the request
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	url := req.URL.String()

	if req.Method == http.MethodPost && strings.HasSuffix(url, "/api/organizations/") {
		jsonResp := `{
			"id": "2e240d2c-78e0-4832-abdc-daa33477a238",
			"name": "test-org",
			"members": [{"email": "test@example.com"}],
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z",
			"subscription": {},
			"tags": {},
			"teams": [{"name": "devops"}],
			"execution_mode": "remote",
			"agents_enabled": true
		}`
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Read (GET to /api/organizations/{id})
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/2e240d2c-78e0-4832-abdc-daa33477a238") {
		jsonResp := `{
			"id": "2e240d2c-78e0-4832-abdc-daa33477a238",
			"name": "test-org",
			"members": [{"email": "test@infradots.com"}],
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z",
			"subscription": {},
			"tags": {},
			"teams": [{"name": "devops"}],
			"execution_mode": "remote",
			"agents_enabled": true
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Import - Get by name (GET to /api/organizations/{name}/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org") && !strings.Contains(url, "/api/organizations/2e240d2c") && !strings.HasSuffix(url, "/api/organizations/") {
		jsonResp := `{
			"id": "2e240d2c-78e0-4832-abdc-daa33477a238",
			"name": "test-org",
			"members": [{"email": "test@infradots.com"}],
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z",
			"subscription": {},
			"tags": {},
			"teams": [{"name": "devops"}],
			"execution_mode": "remote",
			"agents_enabled": true
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle List (GET to /api/organizations/)
	if req.Method == http.MethodGet && strings.HasSuffix(url, "/api/organizations/") {
		jsonResp := `[{
			"id": "2e240d2c-78e0-4832-abdc-daa33477a238",
			"name": "test-org",
			"members": [{"email": "test@infradots.com"}],
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z",
			"subscription": {},
			"tags": {},
			"teams": [{"name": "devops"}],
			"execution_mode": "remote",
			"agents_enabled": true
		}]`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Update (PATCH to /api/organizations/{id})
	if req.Method == http.MethodPatch && strings.Contains(url, "/api/organizations/2e240d2c-78e0-4832-abdc-daa33477a238") {
		jsonResp := `{
			"id": "2e240d2c-78e0-4832-abdc-daa33477a238",
			"name": "updated-org",
			"members": [{"email": "test@infradots.com"}],
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:01:00Z",
			"subscription": {},
			"tags": {},
			"teams": [{"name": "devops"}],
			"execution_mode": "Local",
			"agents_enabled": false
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Delete (DELETE to /api/organizations/{id})
	if req.Method == http.MethodDelete && strings.Contains(url, "/api/organizations/2e240d2c-78e0-4832-abdc-daa33477a238") {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Default: return a 404 Not Found
	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

func setupTestResource(t *testing.T) *OrganizationResource {
	t.Helper()

	httpClient := &http.Client{
		Transport: &MockRoundTripper{},
	}

	provider := &InfradotsProvider{
		host:   "api.infradots.com", // This value is not actually used in tests
		token:  "test-token",
		client: httpClient,
	}

	resource := &OrganizationResource{
		provider: provider,
	}

	return resource
}

func TestOrganizationResource_Create(t *testing.T) {
	r := setupTestResource(t)

	// Create test context
	ctx := context.Background()

	var plan OrganizationResourceModel
	plan.Name = types.StringValue("test-org")

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.CreateRequest{
		Plan: tfsdk.Plan{
			Schema: schemaResp.Schema,
		},
	}
	diags := request.Plan.Set(ctx, &plan)
	require.Empty(t, diags)

	response := resource.CreateResponse{
		State: tfsdk.State{
			Schema: request.Plan.Schema,
		},
	}

	r.Create(ctx, request, &response)

	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	var state OrganizationResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "test-org", state.Name.ValueString())
	assert.Equal(t, "remote", state.ExecutionMode.ValueString())
	assert.True(t, state.AgentsEnabled.ValueBool())
}

func TestOrganizationResource_Read(t *testing.T) {
	r := setupTestResource(t)

	// Create test context
	ctx := context.Background()

	// Setup initial state
	var state OrganizationResourceModel
	state.ID = types.StringValue("2e240d2c-78e0-4832-abdc-daa33477a238")
	state.Name = types.StringValue("test-org")

	// Create request/response objects
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.ReadRequest{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
		},
	}
	// Set the state
	diags := request.State.Set(ctx, &state)
	require.Empty(t, diags)

	response := resource.ReadResponse{
		State: request.State,
	}

	// Call Read
	r.Read(ctx, request, &response)

	// Check for errors
	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	// Parse the response state
	var newState OrganizationResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "2e240d2c-78e0-4832-abdc-daa33477a238", newState.ID.ValueString())
	assert.Equal(t, "test-org", newState.Name.ValueString())
	assert.Equal(t, "2025-07-07T12:00:00Z", newState.CreatedAt.ValueString())
	assert.Equal(t, "2025-07-07T12:00:00Z", newState.UpdatedAt.ValueString())
	assert.Equal(t, "remote", newState.ExecutionMode.ValueString())
	assert.True(t, newState.AgentsEnabled.ValueBool())
}

func TestOrganizationResource_Update(t *testing.T) {
	r := setupTestResource(t)

	// Create test context
	ctx := context.Background()

	// Setup current state
	var state OrganizationResourceModel
	state.ID = types.StringValue("2e240d2c-78e0-4832-abdc-daa33477a238")
	state.Name = types.StringValue("test-org")
	state.ExecutionMode = types.StringValue("remote")
	state.AgentsEnabled = types.BoolValue(true)

	// Setup planned new state
	var plan OrganizationResourceModel
	plan.ID = types.StringValue("2e240d2c-78e0-4832-abdc-daa33477a238")
	plan.Name = types.StringValue("updated-org")
	plan.ExecutionMode = types.StringValue("Local")
	plan.AgentsEnabled = types.BoolValue(false)

	// Create request/response objects
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.UpdateRequest{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
		},
		Plan: tfsdk.Plan{
			Schema: schemaResp.Schema,
		},
	}

	// Set state and plan
	diags := request.State.Set(ctx, &state)
	require.Empty(t, diags)

	diags = request.Plan.Set(ctx, &plan)
	require.Empty(t, diags)

	response := resource.UpdateResponse{
		State: tfsdk.State{
			Schema: request.Plan.Schema,
		},
	}

	// Call Update
	r.Update(ctx, request, &response)

	// Check for errors
	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	var newState OrganizationResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	assert.Equal(t, "2e240d2c-78e0-4832-abdc-daa33477a238", newState.ID.ValueString())
	assert.Equal(t, "updated-org", newState.Name.ValueString())
	assert.Equal(t, "Local", newState.ExecutionMode.ValueString())
	assert.False(t, newState.AgentsEnabled.ValueBool())
	assert.Equal(t, "2025-07-07T12:01:00Z", newState.UpdatedAt.ValueString())
}

func TestOrganizationResource_Delete(t *testing.T) {
	r := setupTestResource(t)

	ctx := context.Background()

	var state OrganizationResourceModel
	state.ID = types.StringValue("2e240d2c-78e0-4832-abdc-daa33477a238")
	state.Name = types.StringValue("test-org")

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.DeleteRequest{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
		},
	}

	diags := request.State.Set(ctx, &state)
	require.Empty(t, diags)

	response := resource.DeleteResponse{
		State: request.State,
	}

	r.Delete(ctx, request, &response)

	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())
}

func TestOrganizationResource_Schema(t *testing.T) {
	r := NewOrganizationResource()

	ctx := context.Background()
	resp := &resource.SchemaResponse{}

	r.Schema(ctx, resource.SchemaRequest{}, resp)

	attrs := resp.Schema.Attributes

	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "name")
	assert.Contains(t, attrs, "created_at")
	assert.Contains(t, attrs, "updated_at")
	assert.Contains(t, attrs, "execution_mode")
	assert.Contains(t, attrs, "agents_enabled")

	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Computed)

	nameAttr := attrs["name"].(schema.StringAttribute)
	assert.True(t, nameAttr.Required)

	createdAtAttr := attrs["created_at"].(schema.StringAttribute)
	assert.True(t, createdAtAttr.Computed)

	executionModeAttr := attrs["execution_mode"].(schema.StringAttribute)
	assert.True(t, executionModeAttr.Computed)
	assert.True(t, executionModeAttr.Optional)

	agentsEnabledAttr := attrs["agents_enabled"].(schema.BoolAttribute)
	assert.True(t, agentsEnabledAttr.Computed)
	assert.True(t, agentsEnabledAttr.Optional)
}

func TestOrganizationResource_ImportState(t *testing.T) {
	r := setupTestResource(t)

	ctx := context.Background()

	// Test successful import
	request := resource.ImportStateRequest{
		ID: "test-org",
	}
	response := resource.ImportStateResponse{
		State: tfsdk.State{},
	}

	// Get schema for state
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	response.State = tfsdk.State{
		Schema: schemaResp.Schema,
	}

	r.ImportState(ctx, request, &response)

	// Check for errors
	require.False(t, response.Diagnostics.HasError())

	// Parse the response state
	var state OrganizationResourceModel
	diags := response.State.Get(ctx, &state)
	require.Empty(t, diags)

	// Verify the imported values
	assert.Equal(t, "2e240d2c-78e0-4832-abdc-daa33477a238", state.ID.ValueString())
	assert.Equal(t, "test-org", state.Name.ValueString())
	assert.Equal(t, "remote", state.ExecutionMode.ValueString())
	assert.True(t, state.AgentsEnabled.ValueBool())
}

func TestOrganizationResource_ImportState_NotFound(t *testing.T) {
	r := setupTestResource(t)

	ctx := context.Background()

	// Test organization not found
	request := resource.ImportStateRequest{
		ID: "nonexistent-org",
	}
	response := resource.ImportStateResponse{
		State: tfsdk.State{},
	}

	// Get schema for state
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	response.State = tfsdk.State{
		Schema: schemaResp.Schema,
	}

	r.ImportState(ctx, request, &response)

	// Should have errors
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Organization not found")
}

func TestOrganizationResource_Metadata(t *testing.T) {
	r := NewOrganizationResource()

	ctx := context.Background()
	resp := &resource.MetadataResponse{}

	r.Metadata(ctx, resource.MetadataRequest{}, resp)

	// Verify the type name
	assert.Equal(t, "infradots_organization", resp.TypeName)
}

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

// MockVCSRoundTripper implements http.RoundTripper for testing VCS resource
type MockVCSRoundTripper struct{}

func (m *MockVCSRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a mocked response based on the request
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	// Check the URL and method to determine response
	url := req.URL.String()

	// Handle Create (POST to /api/organizations/{org_name}/vcs/)
	if req.Method == http.MethodPost && strings.Contains(url, "/api/organizations/test-org/vcs/") {
		jsonResp := `{
			"id": "5f560f5e-0bf3-6543-defg-g1156789012c",
			"name": "test-vcs",
			"vcs_type": "github",
			"url": "https://github.com",
			"clientId": "test-client-id",
			"description": "Test VCS connection for GitHub",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}`
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Read (GET to /api/organizations/{org_name}/vcs/{id})
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/vcs/5f560f5e-0bf3-6543-defg-g1156789012c") {
		jsonResp := `{
			"id": "5f560f5e-0bf3-6543-defg-g1156789012c",
			"name": "test-vcs",
			"vcs_type": "github",
			"url": "https://github.com",
			"clientId": "test-client-id",
			"description": "Test VCS connection for GitHub",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Update (PATCH to /api/organizations/{org_name}/vcs/{id})
	if req.Method == http.MethodPatch && strings.Contains(url, "/api/organizations/test-org/vcs/5f560f5e-0bf3-6543-defg-g1156789012c") {
		jsonResp := `{
			"id": "5f560f5e-0bf3-6543-defg-g1156789012c",
			"name": "updated-vcs",
			"vcs_type": "gitlab",
			"url": "https://gitlab.com",
			"clientId": "updated-client-id",
			"description": "Updated VCS connection for GitLab",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:01:00Z"
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Delete (DELETE to /api/organizations/{org_name}/vcs/{id})
	if req.Method == http.MethodDelete && strings.Contains(url, "/api/organizations/test-org/vcs/5f560f5e-0bf3-6543-defg-g1156789012c") {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Handle Import - List VCS connections (GET to /api/organizations/{org_name}/vcs/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/vcs/") && !strings.Contains(url, "/vcs/5f560f5e") {
		jsonResp := `[{
			"id": "5f560f5e-0bf3-6543-defg-g1156789012c",
			"name": "test-vcs",
			"vcsType": "github",
			"endpoint": "https://github.com",
			"clientId": "test-client-id",
			"description": "Test VCS connection for GitHub",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}]`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Default: return a 404 Not Found
	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

// setupTestVCSResource sets up a test resource with a mock client
func setupTestVCSResource(t *testing.T) *VCSResource {
	t.Helper()

	// Create a client with a mock transport
	httpClient := &http.Client{
		Transport: &MockVCSRoundTripper{},
	}

	// Create a provider with the mock client
	provider := &InfradotsProvider{
		host:   "api.infradots.com", // This value is not actually used in tests
		token:  "test-token",
		client: httpClient,
	}

	resource := &VCSResource{
		provider: provider,
	}

	return resource
}

func TestVCSResource_Create(t *testing.T) {
	r := setupTestVCSResource(t)

	// Create test context
	ctx := context.Background()

	// Setup request with test values
	var plan VCSResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("test-vcs")
	plan.VcsType = types.StringValue("github")
	plan.URL = types.StringValue("https://github.com")
	plan.ClientId = types.StringValue("test-client-id")
	plan.ClientSecret = types.StringValue("test-client-secret")
	plan.Description = types.StringValue("Test VCS connection for GitHub")

	// Create request/response objects
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.CreateRequest{
		Plan: tfsdk.Plan{
			Schema: schemaResp.Schema,
		},
	}
	// Set the plan
	diags := request.Plan.Set(ctx, &plan)
	require.Empty(t, diags)

	response := resource.CreateResponse{
		State: tfsdk.State{
			Schema: request.Plan.Schema,
		},
	}

	// Call Create
	r.Create(ctx, request, &response)

	// Check for errors
	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	// Parse the response state
	var state VCSResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "5f560f5e-0bf3-6543-defg-g1156789012c", state.ID.ValueString())
	assert.Equal(t, "test-vcs", state.Name.ValueString())
	assert.Equal(t, "github", state.VcsType.ValueString())
	assert.Equal(t, "https://github.com", state.URL.ValueString())
	assert.Equal(t, "test-client-id", state.ClientId.ValueString())
	assert.Equal(t, "Test VCS connection for GitHub", state.Description.ValueString())
	assert.Equal(t, "2025-07-07T12:00:00Z", state.CreatedAt.ValueString())
	assert.Equal(t, "2025-07-07T12:00:00Z", state.UpdatedAt.ValueString())
}

func TestVCSResource_Read(t *testing.T) {
	r := setupTestVCSResource(t)

	// Create test context
	ctx := context.Background()

	// Setup initial state
	var state VCSResourceModel
	state.ID = types.StringValue("5f560f5e-0bf3-6543-defg-g1156789012c")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("test-vcs")

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
	var newState VCSResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "5f560f5e-0bf3-6543-defg-g1156789012c", newState.ID.ValueString())
	assert.Equal(t, "test-vcs", newState.Name.ValueString())
	assert.Equal(t, "github", newState.VcsType.ValueString())
	assert.Equal(t, "https://github.com", newState.URL.ValueString())
	assert.Equal(t, "test-client-id", newState.ClientId.ValueString())
	assert.Equal(t, "Test VCS connection for GitHub", newState.Description.ValueString())
}

func TestVCSResource_Update(t *testing.T) {
	r := setupTestVCSResource(t)

	// Create test context
	ctx := context.Background()

	// Setup current state
	var state VCSResourceModel
	state.ID = types.StringValue("5f560f5e-0bf3-6543-defg-g1156789012c")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("test-vcs")
	state.VcsType = types.StringValue("github")
	state.URL = types.StringValue("https://github.com")
	state.ClientId = types.StringValue("test-client-id")
	state.ClientSecret = types.StringValue("test-client-secret")
	state.Description = types.StringValue("Test VCS connection for GitHub")

	// Setup planned new state
	var plan VCSResourceModel
	plan.ID = types.StringValue("5f560f5e-0bf3-6543-defg-g1156789012c")
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("updated-vcs")
	plan.VcsType = types.StringValue("gitlab")
	plan.URL = types.StringValue("https://gitlab.com")
	plan.ClientId = types.StringValue("updated-client-id")
	plan.ClientSecret = types.StringValue("updated-client-secret")
	plan.Description = types.StringValue("Updated VCS connection for GitLab")

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

	// Parse the response state
	var newState VCSResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "5f560f5e-0bf3-6543-defg-g1156789012c", newState.ID.ValueString())
	assert.Equal(t, "updated-vcs", newState.Name.ValueString())
	assert.Equal(t, "gitlab", newState.VcsType.ValueString())
	assert.Equal(t, "https://gitlab.com", newState.URL.ValueString())
	assert.Equal(t, "updated-client-id", newState.ClientId.ValueString())
	assert.Equal(t, "Updated VCS connection for GitLab", newState.Description.ValueString())
	assert.Equal(t, "2025-07-07T12:01:00Z", newState.UpdatedAt.ValueString())
}

func TestVCSResource_Delete(t *testing.T) {
	r := setupTestVCSResource(t)

	// Create test context
	ctx := context.Background()

	// Setup state
	var state VCSResourceModel
	state.ID = types.StringValue("5f560f5e-0bf3-6543-defg-g1156789012c")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("test-vcs")

	// Create request/response objects
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.DeleteRequest{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
		},
	}

	// Set state
	diags := request.State.Set(ctx, &state)
	require.Empty(t, diags)

	// Create a normal response
	response := resource.DeleteResponse{
		State: request.State,
	}

	// Call Delete
	r.Delete(ctx, request, &response)

	// Check for errors
	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	// Resource should be removed from state after Delete
}

func TestVCSResource_Schema(t *testing.T) {
	r := NewVCSResource()

	ctx := context.Background()
	resp := &resource.SchemaResponse{}

	r.Schema(ctx, resource.SchemaRequest{}, resp)

	// Verify the schema attributes
	attrs := resp.Schema.Attributes

	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "name")
	assert.Contains(t, attrs, "vcs_type")
	assert.Contains(t, attrs, "url")
	assert.Contains(t, attrs, "client_id")
	assert.Contains(t, attrs, "client_secret")
	assert.Contains(t, attrs, "description")
	assert.Contains(t, attrs, "created_at")
	assert.Contains(t, attrs, "updated_at")

	// Check specific attribute properties
	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Computed)

	orgNameAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgNameAttr.Required)

	nameAttr := attrs["name"].(schema.StringAttribute)
	assert.True(t, nameAttr.Required)

	vcsTypeAttr := attrs["vcs_type"].(schema.StringAttribute)
	assert.True(t, vcsTypeAttr.Required)

	urlAttr := attrs["url"].(schema.StringAttribute)
	assert.True(t, urlAttr.Required)

	clientIdAttr := attrs["client_id"].(schema.StringAttribute)
	assert.True(t, clientIdAttr.Required)

	clientSecretAttr := attrs["client_secret"].(schema.StringAttribute)
	assert.True(t, clientSecretAttr.Required)
	assert.True(t, clientSecretAttr.Sensitive)

	descAttr := attrs["description"].(schema.StringAttribute)
	assert.True(t, descAttr.Optional)
	assert.True(t, descAttr.Computed)

	createdAtAttr := attrs["created_at"].(schema.StringAttribute)
	assert.True(t, createdAtAttr.Computed)

	updatedAtAttr := attrs["updated_at"].(schema.StringAttribute)
	assert.True(t, updatedAtAttr.Computed)
}

func TestVCSResource_Metadata(t *testing.T) {
	r := NewVCSResource()

	ctx := context.Background()
	resp := &resource.MetadataResponse{}

	r.Metadata(ctx, resource.MetadataRequest{}, resp)

	// Verify the type name
	assert.Equal(t, "infradots_vcs", resp.TypeName)
}

func TestVCSResource_ImportState(t *testing.T) {
	r := setupTestVCSResource(t)

	ctx := context.Background()

	// Test successful import
	request := resource.ImportStateRequest{
		ID: "test-org:test-vcs",
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
	var state VCSResourceModel
	diags := response.State.Get(ctx, &state)
	require.Empty(t, diags)

	// Verify the imported values
	assert.Equal(t, "5f560f5e-0bf3-6543-defg-g1156789012c", state.ID.ValueString())
	assert.Equal(t, "test-org", state.OrganizationName.ValueString())
	assert.Equal(t, "test-vcs", state.Name.ValueString())
	assert.Equal(t, "github", state.VcsType.ValueString())
	assert.Equal(t, "https://github.com", state.URL.ValueString())
	assert.Equal(t, "test-client-id", state.ClientId.ValueString())
}

func TestVCSResource_ImportState_InvalidFormat(t *testing.T) {
	r := setupTestVCSResource(t)

	ctx := context.Background()

	// Test invalid import format
	request := resource.ImportStateRequest{
		ID: "invalid-format",
	}
	response := resource.ImportStateResponse{}

	r.ImportState(ctx, request, &response)

	// Should have errors
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid import ID format")
}

func TestVCSResource_ImportState_NotFound(t *testing.T) {
	r := setupTestVCSResource(t)

	ctx := context.Background()

	// Test VCS not found
	request := resource.ImportStateRequest{
		ID: "test-org:nonexistent-vcs",
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
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "VCS connection not found")
}

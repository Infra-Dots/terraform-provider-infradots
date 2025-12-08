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

// MockVariableRoundTripper implements http.RoundTripper for testing variable resource
type MockVariableRoundTripper struct{}

func (m *MockVariableRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a mocked response based on the request
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	// Check the URL and method to determine response
	url := req.URL.String()

	// Handle Create (POST to /api/organizations/{org_name}/variables/)
	if req.Method == http.MethodPost && strings.Contains(url, "/api/organizations/test-org/variables/") {
		jsonResp := `{
			"id": "4f450f4d-9af2-5432-cdef-f0045678901b",
			"key": "test-variable",
			"value": "test-value",
			"description": "Test variable for Terraform",
			"category": "terraform",
			"sensitive": false,
			"hcl": false,
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}`
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Read (GET to /api/organizations/{org_name}/variables/{id})
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/variables/4f450f4d-9af2-5432-cdef-f0045678901b") {
		jsonResp := `{
			"id": "4f450f4d-9af2-5432-cdef-f0045678901b",
			"key": "test-variable",
			"value": "test-value",
			"description": "Test variable for Terraform",
			"category": "terraform",
			"sensitive": false,
			"hcl": false,
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Update (PATCH to /api/organizations/{org_name}/variables/{id})
	if req.Method == http.MethodPatch && strings.Contains(url, "/api/organizations/test-org/variables/4f450f4d-9af2-5432-cdef-f0045678901b") {
		jsonResp := `{
			"id": "4f450f4d-9af2-5432-cdef-f0045678901b",
			"key": "updated-variable",
			"value": "updated-value",
			"description": "Updated variable description",
			"category": "env",
			"sensitive": true,
			"hcl": true,
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:01:00Z"
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Delete (DELETE to /api/organizations/{org_name}/variables/{id})
	if req.Method == http.MethodDelete && strings.Contains(url, "/api/organizations/test-org/variables/4f450f4d-9af2-5432-cdef-f0045678901b") {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Handle Import - List organization variables (GET to /api/organizations/{org_name}/variables/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/variables/") && !strings.Contains(url, "/variables/4f450f4d") && !strings.Contains(url, "/workspaces/") {
		jsonResp := `[{
			"id": "4f450f4d-9af2-5432-cdef-f0045678901b",
			"key": "test-variable",
			"value": "test-value",
			"description": "Test variable for Terraform",
			"category": "terraform",
			"sensitive": false,
			"hcl": false,
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z",
			"workspace": ""
		}]`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Import - List workspace variables (GET to /api/organizations/{org_name}/workspaces/{workspace_name}/variables/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/workspaces/test-workspace/variables/") {
		jsonResp := `[{
			"id": "5f550f5e-0bf3-6543-defg-g1156789012c",
			"key": "workspace-variable",
			"value": "workspace-value",
			"description": "Test workspace variable",
			"category": "env",
			"sensitive": true,
			"hcl": false,
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z",
			"workspace": "workspace-id-123"
		}]`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Default: return a 404 Not Found
	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

// setupTestVariableResource sets up a test resource with a mock client
func setupTestVariableResource(t *testing.T) *VariableResource {
	t.Helper()

	// Create a client with a mock transport
	httpClient := &http.Client{
		Transport: &MockVariableRoundTripper{},
	}

	// Create a provider with the mock client
	provider := &InfradotsProvider{
		host:   "api.infradots.com", // This value is not actually used in tests
		token:  "test-token",
		client: httpClient,
	}

	resource := &VariableResource{
		provider: provider,
	}

	return resource
}

func TestVariableResource_Create(t *testing.T) {
	r := setupTestVariableResource(t)

	// Create test context
	ctx := context.Background()

	// Setup request with test values
	var plan VariableResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Key = types.StringValue("test-variable")
	plan.Value = types.StringValue("test-value")
	plan.Description = types.StringValue("Test variable for Terraform")
	plan.Category = types.StringValue("terraform")
	plan.Sensitive = types.BoolValue(false)
	plan.HCL = types.BoolValue(false)

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
	var state VariableResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "4f450f4d-9af2-5432-cdef-f0045678901b", state.ID.ValueString())
	assert.Equal(t, "test-variable", state.Key.ValueString())
	assert.Equal(t, "test-value", state.Value.ValueString())
	assert.Equal(t, "Test variable for Terraform", state.Description.ValueString())
	assert.Equal(t, "terraform", state.Category.ValueString())
	assert.False(t, state.Sensitive.ValueBool())
	assert.False(t, state.HCL.ValueBool())
	assert.Equal(t, "2025-07-07T12:00:00Z", state.CreatedAt.ValueString())
	assert.Equal(t, "2025-07-07T12:00:00Z", state.UpdatedAt.ValueString())
}

func TestVariableResource_Read(t *testing.T) {
	r := setupTestVariableResource(t)

	// Create test context
	ctx := context.Background()

	// Setup initial state
	var state VariableResourceModel
	state.ID = types.StringValue("4f450f4d-9af2-5432-cdef-f0045678901b")
	state.OrganizationName = types.StringValue("test-org")
	state.Key = types.StringValue("test-variable")

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
	var newState VariableResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "4f450f4d-9af2-5432-cdef-f0045678901b", newState.ID.ValueString())
	assert.Equal(t, "test-variable", newState.Key.ValueString())
	assert.Equal(t, "test-value", newState.Value.ValueString())
	assert.Equal(t, "Test variable for Terraform", newState.Description.ValueString())
	assert.Equal(t, "terraform", newState.Category.ValueString())
	assert.False(t, newState.Sensitive.ValueBool())
	assert.False(t, newState.HCL.ValueBool())
}

func TestVariableResource_Update(t *testing.T) {
	r := setupTestVariableResource(t)

	// Create test context
	ctx := context.Background()

	// Setup current state
	var state VariableResourceModel
	state.ID = types.StringValue("4f450f4d-9af2-5432-cdef-f0045678901b")
	state.OrganizationName = types.StringValue("test-org")
	state.Key = types.StringValue("test-variable")
	state.Value = types.StringValue("test-value")
	state.Description = types.StringValue("Test variable for Terraform")
	state.Category = types.StringValue("terraform")
	state.Sensitive = types.BoolValue(false)
	state.HCL = types.BoolValue(false)

	// Setup planned new state
	var plan VariableResourceModel
	plan.ID = types.StringValue("4f450f4d-9af2-5432-cdef-f0045678901b")
	plan.OrganizationName = types.StringValue("test-org")
	plan.Key = types.StringValue("updated-variable")
	plan.Value = types.StringValue("updated-value")
	plan.Description = types.StringValue("Updated variable description")
	plan.Category = types.StringValue("env")
	plan.Sensitive = types.BoolValue(true)
	plan.HCL = types.BoolValue(true)

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
	var newState VariableResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "4f450f4d-9af2-5432-cdef-f0045678901b", newState.ID.ValueString())
	assert.Equal(t, "updated-variable", newState.Key.ValueString())
	assert.Equal(t, "updated-value", newState.Value.ValueString())
	assert.Equal(t, "Updated variable description", newState.Description.ValueString())
	assert.Equal(t, "env", newState.Category.ValueString())
	assert.True(t, newState.Sensitive.ValueBool())
	assert.True(t, newState.HCL.ValueBool())
	assert.Equal(t, "2025-07-07T12:01:00Z", newState.UpdatedAt.ValueString())
}

func TestVariableResource_Delete(t *testing.T) {
	r := setupTestVariableResource(t)

	// Create test context
	ctx := context.Background()

	// Setup state
	var state VariableResourceModel
	state.ID = types.StringValue("4f450f4d-9af2-5432-cdef-f0045678901b")
	state.OrganizationName = types.StringValue("test-org")
	state.Key = types.StringValue("test-variable")

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

func TestVariableResource_Schema(t *testing.T) {
	r := NewVariableResource()

	ctx := context.Background()
	resp := &resource.SchemaResponse{}

	r.Schema(ctx, resource.SchemaRequest{}, resp)

	// Verify the schema attributes
	attrs := resp.Schema.Attributes

	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "key")
	assert.Contains(t, attrs, "value")
	assert.Contains(t, attrs, "description")
	assert.Contains(t, attrs, "category")
	assert.Contains(t, attrs, "sensitive")
	assert.Contains(t, attrs, "hcl")
	assert.Contains(t, attrs, "created_at")
	assert.Contains(t, attrs, "updated_at")

	// Check specific attribute properties
	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Computed)

	orgNameAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgNameAttr.Required)

	keyAttr := attrs["key"].(schema.StringAttribute)
	assert.True(t, keyAttr.Required)

	valueAttr := attrs["value"].(schema.StringAttribute)
	assert.True(t, valueAttr.Required)
	assert.True(t, valueAttr.Sensitive)

	descAttr := attrs["description"].(schema.StringAttribute)
	assert.True(t, descAttr.Optional)
	assert.True(t, descAttr.Computed)

	categoryAttr := attrs["category"].(schema.StringAttribute)
	assert.True(t, categoryAttr.Optional)
	assert.True(t, categoryAttr.Computed)

	sensitiveAttr := attrs["sensitive"].(schema.BoolAttribute)
	assert.True(t, sensitiveAttr.Optional)
	assert.True(t, sensitiveAttr.Computed)

	hclAttr := attrs["hcl"].(schema.BoolAttribute)
	assert.True(t, hclAttr.Optional)
	assert.True(t, hclAttr.Computed)

	createdAtAttr := attrs["created_at"].(schema.StringAttribute)
	assert.True(t, createdAtAttr.Computed)

	updatedAtAttr := attrs["updated_at"].(schema.StringAttribute)
	assert.True(t, updatedAtAttr.Computed)
}

func TestVariableResource_Metadata(t *testing.T) {
	r := NewVariableResource()

	ctx := context.Background()
	resp := &resource.MetadataResponse{}

	r.Metadata(ctx, resource.MetadataRequest{}, resp)

	// Verify the type name
	assert.Equal(t, "infradots_variable", resp.TypeName)
}

func TestVariableResource_ImportState_OrganizationVariable(t *testing.T) {
	r := setupTestVariableResource(t)

	ctx := context.Background()

	// Test successful import of organization-level variable
	request := resource.ImportStateRequest{
		ID: "test-org:test-variable",
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
	var state VariableResourceModel
	diags := response.State.Get(ctx, &state)
	require.Empty(t, diags)

	// Verify the imported values
	assert.Equal(t, "4f450f4d-9af2-5432-cdef-f0045678901b", state.ID.ValueString())
	assert.Equal(t, "test-org", state.OrganizationName.ValueString())
	assert.Equal(t, "test-variable", state.Key.ValueString())
	assert.Equal(t, "test-value", state.Value.ValueString())
	assert.Equal(t, "terraform", state.Category.ValueString())
	assert.False(t, state.Sensitive.ValueBool())
	assert.False(t, state.HCL.ValueBool())
	assert.True(t, state.Workspace.IsNull())
}

func TestVariableResource_ImportState_WorkspaceVariable(t *testing.T) {
	r := setupTestVariableResource(t)

	ctx := context.Background()

	// Test successful import of workspace-level variable
	request := resource.ImportStateRequest{
		ID: "test-org:test-workspace:workspace-variable",
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
	var state VariableResourceModel
	diags := response.State.Get(ctx, &state)
	require.Empty(t, diags)

	// Verify the imported values
	assert.Equal(t, "5f550f5e-0bf3-6543-defg-g1156789012c", state.ID.ValueString())
	assert.Equal(t, "test-org", state.OrganizationName.ValueString())
	assert.Equal(t, "workspace-variable", state.Key.ValueString())
	assert.Equal(t, "workspace-value", state.Value.ValueString())
	assert.Equal(t, "env", state.Category.ValueString())
	assert.True(t, state.Sensitive.ValueBool())
	assert.False(t, state.HCL.ValueBool())
	assert.Equal(t, "workspace-id-123", state.Workspace.ValueString())
}

func TestVariableResource_ImportState_InvalidFormat(t *testing.T) {
	r := setupTestVariableResource(t)

	ctx := context.Background()

	// Test invalid import format (only 1 part)
	request := resource.ImportStateRequest{
		ID: "invalid-format",
	}
	response := resource.ImportStateResponse{}

	r.ImportState(ctx, request, &response)

	// Should have errors
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid import ID format")

	// Test invalid import format (4 parts)
	request.ID = "org:workspace:key:extra"
	response = resource.ImportStateResponse{}
	r.ImportState(ctx, request, &response)

	// Should have errors
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid import ID format")
}

func TestVariableResource_ImportState_NotFound(t *testing.T) {
	r := setupTestVariableResource(t)

	ctx := context.Background()

	// Test variable not found (organization level)
	request := resource.ImportStateRequest{
		ID: "test-org:nonexistent-variable",
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
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Variable not found")
}

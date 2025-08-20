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

// MockWorkspaceRoundTripper implements http.RoundTripper for testing workspace resource
type MockWorkspaceRoundTripper struct{}

func (m *MockWorkspaceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a mocked response based on the request
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	// Check the URL and method to determine response
	url := req.URL.String()

	// Handle Create (POST to /api/organizations/{org_name}/workspaces/)
	if req.Method == http.MethodPost && strings.Contains(url, "/api/organizations/test-org/workspaces/") {
		jsonResp := `{
			"id": "3f340e3c-89f1-4321-bcde-eff34567890a",
			"name": "test-workspace",
			"description": "Test workspace for Terraform",
			"source": "https://github.com/test/repo",
			"branch": "main",
			"terraform_version": "1.5.0",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}`
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Read (GET to /api/organizations/{org_name}/workspaces/{id})
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/workspaces/3f340e3c-89f1-4321-bcde-eff34567890a") {
		jsonResp := `{
			"id": "3f340e3c-89f1-4321-bcde-eff34567890a",
			"name": "test-workspace",
			"description": "Test workspace for Terraform",
			"source": "https://github.com/test/repo",
			"branch": "main",
			"terraform_version": "1.5.0",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Update (PATCH to /api/organizations/{org_name}/workspaces/{id})
	if req.Method == http.MethodPatch && strings.Contains(url, "/api/organizations/test-org/workspaces/3f340e3c-89f1-4321-bcde-eff34567890a") {
		jsonResp := `{
			"id": "3f340e3c-89f1-4321-bcde-eff34567890a",
			"name": "updated-workspace",
			"description": "Updated workspace description",
			"source": "https://github.com/test/updated-repo",
			"branch": "develop",
			"terraform_version": "1.6.0",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:01:00Z"
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Delete (DELETE to /api/organizations/{org_name}/workspaces/{id})
	if req.Method == http.MethodDelete && strings.Contains(url, "/api/organizations/test-org/workspaces/3f340e3c-89f1-4321-bcde-eff34567890a") {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Default: return a 404 Not Found
	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

// setupTestWorkspaceResource sets up a test resource with a mock client
func setupTestWorkspaceResource(t *testing.T) *WorkspaceResource {
	t.Helper()

	// Create a client with a mock transport
	httpClient := &http.Client{
		Transport: &MockWorkspaceRoundTripper{},
	}

	// Create a provider with the mock client
	provider := &InfradotsProvider{
		host:   "api.infradots.com", // This value is not actually used in tests
		token:  "test-token",
		client: httpClient,
	}

	resource := &WorkspaceResource{
		provider: provider,
	}

	return resource
}

func TestWorkspaceResource_Create(t *testing.T) {
	r := setupTestWorkspaceResource(t)

	// Create test context
	ctx := context.Background()

	// Setup request with test values
	var plan WorkspaceResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("test-workspace")
	plan.Description = types.StringValue("Test workspace for Terraform")
	plan.Source = types.StringValue("https://github.com/test/repo")
	plan.Branch = types.StringValue("main")
	plan.TerraformVersion = types.StringValue("1.5.0")

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
	var state WorkspaceResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "3f340e3c-89f1-4321-bcde-eff34567890a", state.ID.ValueString())
	assert.Equal(t, "test-workspace", state.Name.ValueString())
	assert.Equal(t, "Test workspace for Terraform", state.Description.ValueString())
	assert.Equal(t, "https://github.com/test/repo", state.Source.ValueString())
	assert.Equal(t, "main", state.Branch.ValueString())
	assert.Equal(t, "1.5.0", state.TerraformVersion.ValueString())
	assert.Equal(t, "2025-07-07T12:00:00Z", state.CreatedAt.ValueString())
	assert.Equal(t, "2025-07-07T12:00:00Z", state.UpdatedAt.ValueString())
}

func TestWorkspaceResource_Read(t *testing.T) {
	r := setupTestWorkspaceResource(t)

	// Create test context
	ctx := context.Background()

	// Setup initial state
	var state WorkspaceResourceModel
	state.ID = types.StringValue("3f340e3c-89f1-4321-bcde-eff34567890a")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("test-workspace")

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
	var newState WorkspaceResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "3f340e3c-89f1-4321-bcde-eff34567890a", newState.ID.ValueString())
	assert.Equal(t, "test-workspace", newState.Name.ValueString())
	assert.Equal(t, "Test workspace for Terraform", newState.Description.ValueString())
	assert.Equal(t, "https://github.com/test/repo", newState.Source.ValueString())
	assert.Equal(t, "main", newState.Branch.ValueString())
	assert.Equal(t, "1.5.0", newState.TerraformVersion.ValueString())
}

func TestWorkspaceResource_Update(t *testing.T) {
	r := setupTestWorkspaceResource(t)

	// Create test context
	ctx := context.Background()

	// Setup current state
	var state WorkspaceResourceModel
	state.ID = types.StringValue("3f340e3c-89f1-4321-bcde-eff34567890a")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("test-workspace")
	state.Description = types.StringValue("Test workspace for Terraform")
	state.Source = types.StringValue("https://github.com/test/repo")
	state.Branch = types.StringValue("main")
	state.TerraformVersion = types.StringValue("1.5.0")

	// Setup planned new state
	var plan WorkspaceResourceModel
	plan.ID = types.StringValue("3f340e3c-89f1-4321-bcde-eff34567890a")
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("updated-workspace")
	plan.Description = types.StringValue("Updated workspace description")
	plan.Source = types.StringValue("https://github.com/test/updated-repo")
	plan.Branch = types.StringValue("develop")
	plan.TerraformVersion = types.StringValue("1.6.0")

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
	var newState WorkspaceResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	// Verify the values
	assert.Equal(t, "3f340e3c-89f1-4321-bcde-eff34567890a", newState.ID.ValueString())
	assert.Equal(t, "updated-workspace", newState.Name.ValueString())
	assert.Equal(t, "Updated workspace description", newState.Description.ValueString())
	assert.Equal(t, "https://github.com/test/updated-repo", newState.Source.ValueString())
	assert.Equal(t, "develop", newState.Branch.ValueString())
	assert.Equal(t, "1.6.0", newState.TerraformVersion.ValueString())
	assert.Equal(t, "2025-07-07T12:01:00Z", newState.UpdatedAt.ValueString())
}

func TestWorkspaceResource_Delete(t *testing.T) {
	r := setupTestWorkspaceResource(t)

	// Create test context
	ctx := context.Background()

	// Setup state
	var state WorkspaceResourceModel
	state.ID = types.StringValue("3f340e3c-89f1-4321-bcde-eff34567890a")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("test-workspace")

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

func TestWorkspaceResource_Schema(t *testing.T) {
	r := NewWorkspaceResource()

	ctx := context.Background()
	resp := &resource.SchemaResponse{}

	r.Schema(ctx, resource.SchemaRequest{}, resp)

	// Verify the schema attributes
	attrs := resp.Schema.Attributes

	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "name")
	assert.Contains(t, attrs, "description")
	assert.Contains(t, attrs, "source")
	assert.Contains(t, attrs, "branch")
	assert.Contains(t, attrs, "terraform_version")
	assert.Contains(t, attrs, "created_at")
	assert.Contains(t, attrs, "updated_at")

	// Check specific attribute properties
	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Computed)

	orgNameAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgNameAttr.Required)

	nameAttr := attrs["name"].(schema.StringAttribute)
	assert.True(t, nameAttr.Required)

	descAttr := attrs["description"].(schema.StringAttribute)
	assert.True(t, descAttr.Optional)

	sourceAttr := attrs["source"].(schema.StringAttribute)
	assert.True(t, sourceAttr.Required)

	branchAttr := attrs["branch"].(schema.StringAttribute)
	assert.True(t, branchAttr.Required)

	tfVersionAttr := attrs["terraform_version"].(schema.StringAttribute)
	assert.True(t, tfVersionAttr.Required)

	createdAtAttr := attrs["created_at"].(schema.StringAttribute)
	assert.True(t, createdAtAttr.Computed)

	updatedAtAttr := attrs["updated_at"].(schema.StringAttribute)
	assert.True(t, updatedAtAttr.Computed)
}

func TestWorkspaceResource_Metadata(t *testing.T) {
	r := NewWorkspaceResource()

	ctx := context.Background()
	resp := &resource.MetadataResponse{}

	r.Metadata(ctx, resource.MetadataRequest{}, resp)

	// Verify the type name
	assert.Equal(t, "infradots_workspace", resp.TypeName)
}

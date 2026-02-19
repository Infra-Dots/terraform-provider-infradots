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

type MockPermissionRoundTripper struct{}

func (m *MockPermissionRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	url := req.URL.String()

	// Handle Create org-level permission (POST /api/permissions/{org}/)
	if req.Method == http.MethodPost && strings.Contains(url, "/api/permissions/test-org/") && !strings.Contains(url, "/workspaces/") {
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(`{"message": "Permission created"}`))
		return resp, nil
	}

	// Handle Create workspace-level permission (POST /api/permissions/{org}/workspaces/)
	if req.Method == http.MethodPost && strings.Contains(url, "/api/permissions/test-org/workspaces/") {
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(`{"message": "Permission created"}`))
		return resp, nil
	}

	// Handle Read org-level permissions (GET /api/permissions/{org}/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/permissions/test-org/") && !strings.Contains(url, "/workspaces/") {
		jsonResp := `[{
			"permission": "read_workspaces",
			"user": "user@example.com",
			"organization": "test-org"
		}]`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Delete permission (DELETE /api/permissions/{org}/)
	if req.Method == http.MethodDelete {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

func setupTestPermissionResource(t *testing.T) *PermissionResource {
	t.Helper()
	httpClient := &http.Client{Transport: &MockPermissionRoundTripper{}}
	provider := &InfradotsProvider{
		host:   "api.infradots.com",
		token:  "test-token",
		client: httpClient,
	}
	return &PermissionResource{provider: provider}
}

func TestPermissionResource_Metadata(t *testing.T) {
	r := NewPermissionResource()
	ctx := context.Background()
	resp := &resource.MetadataResponse{}
	r.Metadata(ctx, resource.MetadataRequest{}, resp)
	assert.Equal(t, "infradots_permission", resp.TypeName)
}

func TestPermissionResource_Schema(t *testing.T) {
	r := NewPermissionResource()
	ctx := context.Background()
	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)

	attrs := resp.Schema.Attributes
	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "team_id")
	assert.Contains(t, attrs, "user_email")
	assert.Contains(t, attrs, "permission")
	assert.Contains(t, attrs, "workspace_name")

	orgAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgAttr.Required)

	permAttr := attrs["permission"].(schema.StringAttribute)
	assert.True(t, permAttr.Required)

	teamAttr := attrs["team_id"].(schema.StringAttribute)
	assert.True(t, teamAttr.Optional)

	userAttr := attrs["user_email"].(schema.StringAttribute)
	assert.True(t, userAttr.Optional)

	wsAttr := attrs["workspace_name"].(schema.StringAttribute)
	assert.True(t, wsAttr.Optional)
}

func TestPermissionResource_Create(t *testing.T) {
	r := setupTestPermissionResource(t)
	ctx := context.Background()

	var plan PermissionResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.UserEmail = types.StringValue("user@example.com")
	plan.Permission = types.StringValue("read_workspaces")
	plan.TeamID = types.StringNull()
	plan.WorkspaceName = types.StringNull()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema},
	}
	diags := request.Plan.Set(ctx, &plan)
	require.Empty(t, diags)

	response := resource.CreateResponse{
		State: tfsdk.State{Schema: request.Plan.Schema},
	}

	r.Create(ctx, request, &response)

	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	var state PermissionResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "test-org", state.OrganizationName.ValueString())
	assert.Equal(t, "read_workspaces", state.Permission.ValueString())
	assert.Equal(t, "user@example.com", state.UserEmail.ValueString())
	assert.NotEmpty(t, state.ID.ValueString())
}

func TestPermissionResource_Read(t *testing.T) {
	r := setupTestPermissionResource(t)
	ctx := context.Background()

	var state PermissionResourceModel
	state.ID = types.StringValue("test-org:read_workspaces:user:user@example.com")
	state.OrganizationName = types.StringValue("test-org")
	state.UserEmail = types.StringValue("user@example.com")
	state.Permission = types.StringValue("read_workspaces")
	state.TeamID = types.StringNull()
	state.WorkspaceName = types.StringNull()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	diags := request.State.Set(ctx, &state)
	require.Empty(t, diags)

	response := resource.ReadResponse{State: request.State}

	r.Read(ctx, request, &response)

	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())
}

func TestPermissionResource_Delete(t *testing.T) {
	r := setupTestPermissionResource(t)
	ctx := context.Background()

	var state PermissionResourceModel
	state.ID = types.StringValue("test-org:read_workspaces:user:user@example.com")
	state.OrganizationName = types.StringValue("test-org")
	state.UserEmail = types.StringValue("user@example.com")
	state.Permission = types.StringValue("read_workspaces")
	state.TeamID = types.StringNull()
	state.WorkspaceName = types.StringNull()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	diags := request.State.Set(ctx, &state)
	require.Empty(t, diags)

	response := resource.DeleteResponse{State: request.State}

	r.Delete(ctx, request, &response)

	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())
}

func TestPermissionResource_Create_NoUserOrTeam(t *testing.T) {
	r := setupTestPermissionResource(t)
	ctx := context.Background()

	var plan PermissionResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Permission = types.StringValue("read_workspaces")
	plan.UserEmail = types.StringNull()
	plan.TeamID = types.StringNull()
	plan.WorkspaceName = types.StringNull()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema},
	}
	diags := request.Plan.Set(ctx, &plan)
	require.Empty(t, diags)

	response := resource.CreateResponse{
		State: tfsdk.State{Schema: request.Plan.Schema},
	}

	r.Create(ctx, request, &response)

	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid configuration")
}

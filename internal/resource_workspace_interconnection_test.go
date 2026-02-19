package internal

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockInterconnectionRoundTripper struct{}

func (m *MockInterconnectionRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	url := req.URL.String()

	// Handle Connect (POST /api/organizations/{org}/workspaces/{ws}/connect_workspaces/)
	if req.Method == http.MethodPost && strings.Contains(url, "/connect_workspaces/") {
		jsonResp := `{
			"connected": ["ws-staging", "ws-production"],
			"workspace_not_existing": []
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle List Connected (GET /api/organizations/{org}/workspaces/{ws}/connect_workspaces/)
	if req.Method == http.MethodGet && strings.Contains(url, "/connect_workspaces/") {
		jsonResp := `{
			"id": 1,
			"condition": "full_apply",
			"connected_workspaces": [
				{"id": "ws-id-1", "name": "ws-staging"},
				{"id": "ws-id-2", "name": "ws-production"}
			]
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Disconnect (DELETE /api/organizations/{org}/workspaces/{ws}/connect_workspaces/)
	if req.Method == http.MethodDelete && strings.Contains(url, "/connect_workspaces/") {
		jsonResp := `{"disconnected": ["ws-staging", "ws-production"]}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

func setupTestInterconnectionResource(t *testing.T) *WorkspaceInterconnectionResource {
	t.Helper()
	httpClient := &http.Client{Transport: &MockInterconnectionRoundTripper{}}
	provider := &InfradotsProvider{
		host:   "api.infradots.com",
		token:  "test-token",
		client: httpClient,
	}
	return &WorkspaceInterconnectionResource{provider: provider}
}

func TestWorkspaceInterconnectionResource_Metadata(t *testing.T) {
	r := NewWorkspaceInterconnectionResource()
	ctx := context.Background()
	resp := &resource.MetadataResponse{}
	r.Metadata(ctx, resource.MetadataRequest{}, resp)
	assert.Equal(t, "infradots_workspace_interconnection", resp.TypeName)
}

func TestWorkspaceInterconnectionResource_Schema(t *testing.T) {
	r := NewWorkspaceInterconnectionResource()
	ctx := context.Background()
	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)

	attrs := resp.Schema.Attributes
	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "workspace_name")
	assert.Contains(t, attrs, "connected_to")
	assert.Contains(t, attrs, "condition")

	orgAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgAttr.Required)

	wsAttr := attrs["workspace_name"].(schema.StringAttribute)
	assert.True(t, wsAttr.Required)

	condAttr := attrs["condition"].(schema.StringAttribute)
	assert.True(t, condAttr.Optional)
	assert.True(t, condAttr.Computed)
}

func TestWorkspaceInterconnectionResource_Create(t *testing.T) {
	r := setupTestInterconnectionResource(t)
	ctx := context.Background()

	var plan WorkspaceInterconnectionResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.WorkspaceName = types.StringValue("ws-main")
	plan.ConnectedTo = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("ws-staging"),
		types.StringValue("ws-production"),
	})
	plan.Condition = types.StringValue("full_apply")

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

	var state WorkspaceInterconnectionResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "test-org:ws-main", state.ID.ValueString())
	assert.Equal(t, "full_apply", state.Condition.ValueString())
	assert.Equal(t, 2, len(state.ConnectedTo.Elements()))
}

func TestWorkspaceInterconnectionResource_Read(t *testing.T) {
	r := setupTestInterconnectionResource(t)
	ctx := context.Background()

	var state WorkspaceInterconnectionResourceModel
	state.ID = types.StringValue("test-org:ws-main")
	state.OrganizationName = types.StringValue("test-org")
	state.WorkspaceName = types.StringValue("ws-main")
	state.ConnectedTo = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("ws-staging"),
	})
	state.Condition = types.StringValue("full_apply")

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

	var newState WorkspaceInterconnectionResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	assert.Equal(t, "full_apply", newState.Condition.ValueString())
	assert.Equal(t, 2, len(newState.ConnectedTo.Elements()))
}

func TestWorkspaceInterconnectionResource_Delete(t *testing.T) {
	r := setupTestInterconnectionResource(t)
	ctx := context.Background()

	var state WorkspaceInterconnectionResourceModel
	state.ID = types.StringValue("test-org:ws-main")
	state.OrganizationName = types.StringValue("test-org")
	state.WorkspaceName = types.StringValue("ws-main")
	state.ConnectedTo = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("ws-staging"),
		types.StringValue("ws-production"),
	})
	state.Condition = types.StringValue("full_apply")

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

func TestWorkspaceInterconnectionResource_ImportState(t *testing.T) {
	r := setupTestInterconnectionResource(t)
	ctx := context.Background()

	request := resource.ImportStateRequest{ID: "test-org:ws-main"}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	response := resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.ImportState(ctx, request, &response)
	require.False(t, response.Diagnostics.HasError())

	var state WorkspaceInterconnectionResourceModel
	diags := response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "test-org:ws-main", state.ID.ValueString())
	assert.Equal(t, "full_apply", state.Condition.ValueString())
	assert.Equal(t, 2, len(state.ConnectedTo.Elements()))
}

func TestWorkspaceInterconnectionResource_ImportState_InvalidFormat(t *testing.T) {
	r := setupTestInterconnectionResource(t)
	ctx := context.Background()

	request := resource.ImportStateRequest{ID: "invalid"}
	response := resource.ImportStateResponse{}

	r.ImportState(ctx, request, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid import ID format")
}

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

type MockTeamRoundTripper struct{}

func (m *MockTeamRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	url := req.URL.String()

	// Handle Create (POST /api/organizations/{org}/teams/)
	if req.Method == http.MethodPost && strings.Contains(url, "/api/organizations/test-org/teams/") && !strings.Contains(url, "/members") {
		jsonResp := `{
			"id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"name": "devops-team",
			"members": [{"email": "user1@example.com"}, {"email": "user2@example.com"}],
			"permissions": []
		}`
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Update Members (POST /api/organizations/{org}/teams/{id}/members/)
	if req.Method == http.MethodPost && strings.Contains(url, "/members/") {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Handle Read (GET /api/organizations/{org}/teams/{id}/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/teams/a1b2c3d4-e5f6-7890-abcd-ef1234567890") {
		jsonResp := `{
			"id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"name": "devops-team",
			"members": [{"email": "user1@example.com"}, {"email": "user2@example.com"}],
			"permissions": []
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Update (PATCH /api/organizations/{org}/teams/{id}/)
	if req.Method == http.MethodPatch && strings.Contains(url, "/api/organizations/test-org/teams/a1b2c3d4-e5f6-7890-abcd-ef1234567890") {
		resp.StatusCode = http.StatusOK
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Handle Delete (DELETE /api/organizations/{org}/teams/{id}/)
	if req.Method == http.MethodDelete && strings.Contains(url, "/api/organizations/test-org/teams/a1b2c3d4-e5f6-7890-abcd-ef1234567890") {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Handle List (GET /api/organizations/{org}/teams/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/teams/") {
		jsonResp := `[{
			"id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"name": "devops-team",
			"members": [{"email": "user1@example.com"}],
			"permissions": []
		}]`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

func setupTestTeamResource(t *testing.T) *TeamResource {
	t.Helper()
	httpClient := &http.Client{Transport: &MockTeamRoundTripper{}}
	provider := &InfradotsProvider{
		host:   "api.infradots.com",
		token:  "test-token",
		client: httpClient,
	}
	return &TeamResource{provider: provider}
}

func TestTeamResource_Metadata(t *testing.T) {
	r := NewTeamResource()
	ctx := context.Background()
	resp := &resource.MetadataResponse{}
	r.Metadata(ctx, resource.MetadataRequest{}, resp)
	assert.Equal(t, "infradots_team", resp.TypeName)
}

func TestTeamResource_Schema(t *testing.T) {
	r := NewTeamResource()
	ctx := context.Background()
	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)

	attrs := resp.Schema.Attributes
	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "name")
	assert.Contains(t, attrs, "members")

	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Computed)

	orgAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgAttr.Required)

	nameAttr := attrs["name"].(schema.StringAttribute)
	assert.True(t, nameAttr.Required)
}

func TestTeamResource_Create(t *testing.T) {
	r := setupTestTeamResource(t)
	ctx := context.Background()

	var plan TeamResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("devops-team")
	plan.Members = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("user1@example.com"),
		types.StringValue("user2@example.com"),
	})

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

	var state TeamResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", state.ID.ValueString())
	assert.Equal(t, "devops-team", state.Name.ValueString())
	assert.Equal(t, "test-org", state.OrganizationName.ValueString())
}

func TestTeamResource_Read(t *testing.T) {
	r := setupTestTeamResource(t)
	ctx := context.Background()

	var state TeamResourceModel
	state.ID = types.StringValue("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("devops-team")
	state.Members = types.ListValueMust(types.StringType, []attr.Value{})

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

	var newState TeamResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", newState.ID.ValueString())
	assert.Equal(t, "devops-team", newState.Name.ValueString())
}

func TestTeamResource_Delete(t *testing.T) {
	r := setupTestTeamResource(t)
	ctx := context.Background()

	var state TeamResourceModel
	state.ID = types.StringValue("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("devops-team")
	state.Members = types.ListValueMust(types.StringType, []attr.Value{})

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

func TestTeamResource_ImportState(t *testing.T) {
	r := setupTestTeamResource(t)
	ctx := context.Background()

	request := resource.ImportStateRequest{ID: "test-org:devops-team"}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	response := resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.ImportState(ctx, request, &response)
	require.False(t, response.Diagnostics.HasError())

	var state TeamResourceModel
	diags := response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", state.ID.ValueString())
	assert.Equal(t, "test-org", state.OrganizationName.ValueString())
	assert.Equal(t, "devops-team", state.Name.ValueString())
}

func TestTeamResource_ImportState_InvalidFormat(t *testing.T) {
	r := setupTestTeamResource(t)
	ctx := context.Background()

	request := resource.ImportStateRequest{ID: "invalid-format"}
	response := resource.ImportStateResponse{}

	r.ImportState(ctx, request, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid import ID format")
}

func TestTeamResource_ImportState_NotFound(t *testing.T) {
	r := setupTestTeamResource(t)
	ctx := context.Background()

	request := resource.ImportStateRequest{ID: "test-org:nonexistent-team"}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	response := resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.ImportState(ctx, request, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Team not found")
}

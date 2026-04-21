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

// MockAgentSkillRoundTripper implements http.RoundTripper for testing agent skill resource
type MockAgentSkillRoundTripper struct{}

func (m *MockAgentSkillRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	url := req.URL.String()

	// Handle Create (POST to /api/organizations/{org}/skills/)
	if req.Method == http.MethodPost && strings.Contains(url, "/api/agents/test-org/skills/") {
		jsonResp := `{
			"id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"name": "review",
			"display_name": "Code Review",
			"description": "Reviews Terraform plans before apply",
			"enabled": true,
			"config": {"model": "claude-sonnet-4-6", "temperature": 0.3},
			"source_repo": "",
			"source_path": "",
			"source_ref": "",
			"is_github_sourced": false,
			"created_at": "2025-07-07T12:00:00Z",
			"created_by": null
		}`
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Read (GET to /api/organizations/{org}/skills/{id})
	if req.Method == http.MethodGet && strings.Contains(url, "/api/agents/test-org/skills/a1b2c3d4-e5f6-7890-abcd-ef1234567890") {
		jsonResp := `{
			"id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"name": "review",
			"display_name": "Code Review",
			"description": "Reviews Terraform plans before apply",
			"enabled": true,
			"config": {"model": "claude-sonnet-4-6", "temperature": 0.3},
			"source_repo": "",
			"source_path": "",
			"source_ref": "",
			"is_github_sourced": false,
			"created_at": "2025-07-07T12:00:00Z",
			"created_by": null
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Update (PATCH to /api/organizations/{org}/skills/{id})
	if req.Method == http.MethodPatch && strings.Contains(url, "/api/agents/test-org/skills/a1b2c3d4-e5f6-7890-abcd-ef1234567890") {
		jsonResp := `{
			"id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"name": "review",
			"display_name": "Updated Code Review",
			"description": "Updated description",
			"enabled": false,
			"config": {"model": "claude-opus-4-6"},
			"source_repo": "https://github.com/example/skills",
			"source_path": "skills/review",
			"source_ref": "main",
			"is_github_sourced": true,
			"created_at": "2025-07-07T12:00:00Z",
			"created_by": null
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Delete (DELETE to /api/organizations/{org}/skills/{id})
	if req.Method == http.MethodDelete && strings.Contains(url, "/api/agents/test-org/skills/a1b2c3d4-e5f6-7890-abcd-ef1234567890") {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Default: 404
	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

func setupTestAgentSkillResource(t *testing.T) *AgentSkillResource {
	t.Helper()

	httpClient := &http.Client{
		Transport: &MockAgentSkillRoundTripper{},
	}

	provider := &InfradotsProvider{
		host:   "api.infradots.com",
		token:  "test-token",
		client: httpClient,
	}

	return &AgentSkillResource{provider: provider}
}

func TestAgentSkillResource_Metadata(t *testing.T) {
	r := NewAgentSkillResource()

	ctx := context.Background()
	resp := &resource.MetadataResponse{}

	r.Metadata(ctx, resource.MetadataRequest{}, resp)

	assert.Equal(t, "infradots_agent_skill", resp.TypeName)
}

func TestAgentSkillResource_Schema(t *testing.T) {
	r := NewAgentSkillResource()

	ctx := context.Background()
	resp := &resource.SchemaResponse{}

	r.Schema(ctx, resource.SchemaRequest{}, resp)

	attrs := resp.Schema.Attributes

	// Check all expected attributes exist
	expectedAttrs := []string{
		"id", "organization_name", "name", "display_name", "description",
		"enabled", "config", "source_repo", "source_path", "source_ref",
		"is_github_sourced", "created_at",
	}
	for _, attr := range expectedAttrs {
		assert.Contains(t, attrs, attr)
	}

	// Check specific attribute properties
	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Computed)

	orgNameAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgNameAttr.Required)

	nameAttr := attrs["name"].(schema.StringAttribute)
	assert.True(t, nameAttr.Required)

	displayNameAttr := attrs["display_name"].(schema.StringAttribute)
	assert.True(t, displayNameAttr.Required)

	descAttr := attrs["description"].(schema.StringAttribute)
	assert.True(t, descAttr.Optional)
	assert.True(t, descAttr.Computed)

	enabledAttr := attrs["enabled"].(schema.BoolAttribute)
	assert.True(t, enabledAttr.Optional)
	assert.True(t, enabledAttr.Computed)

	configAttr := attrs["config"].(schema.StringAttribute)
	assert.True(t, configAttr.Optional)
	assert.True(t, configAttr.Computed)

	isGHAttr := attrs["is_github_sourced"].(schema.BoolAttribute)
	assert.True(t, isGHAttr.Computed)

	createdAtAttr := attrs["created_at"].(schema.StringAttribute)
	assert.True(t, createdAtAttr.Computed)
}

func TestAgentSkillResource_Create(t *testing.T) {
	r := setupTestAgentSkillResource(t)
	ctx := context.Background()

	var plan AgentSkillResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("review")
	plan.DisplayName = types.StringValue("Code Review")
	plan.Description = types.StringValue("Reviews Terraform plans before apply")
	plan.Enabled = types.BoolValue(true)
	plan.Config = types.StringValue(`{"model": "claude-sonnet-4-6", "temperature": 0.3}`)
	plan.SourceRepo = types.StringValue("")
	plan.SourcePath = types.StringValue("")
	plan.SourceRef = types.StringValue("")

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema},
	}
	diags := request.Plan.Set(ctx, &plan)
	require.Empty(t, diags)

	response := resource.CreateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.Create(ctx, request, &response)

	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	var state AgentSkillResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", state.ID.ValueString())
	assert.Equal(t, "review", state.Name.ValueString())
	assert.Equal(t, "Code Review", state.DisplayName.ValueString())
	assert.Equal(t, "Reviews Terraform plans before apply", state.Description.ValueString())
	assert.True(t, state.Enabled.ValueBool())
	assert.False(t, state.IsGithubSourced.ValueBool())
	assert.Equal(t, "2025-07-07T12:00:00Z", state.CreatedAt.ValueString())
}

func TestAgentSkillResource_Read(t *testing.T) {
	r := setupTestAgentSkillResource(t)
	ctx := context.Background()

	var state AgentSkillResourceModel
	state.ID = types.StringValue("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("review")
	state.DisplayName = types.StringValue("Code Review")
	state.Description = types.StringValue("Reviews Terraform plans before apply")
	state.Enabled = types.BoolValue(true)
	state.Config = types.StringValue("")
	state.SourceRepo = types.StringValue("")
	state.SourcePath = types.StringValue("")
	state.SourceRef = types.StringValue("")
	state.IsGithubSourced = types.BoolValue(false)
	state.CreatedAt = types.StringValue("2025-07-07T12:00:00Z")

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.ReadRequest{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	diags := request.State.Set(ctx, &state)
	require.Empty(t, diags)

	response := resource.ReadResponse{
		State: request.State,
	}

	r.Read(ctx, request, &response)

	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	var newState AgentSkillResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", newState.ID.ValueString())
	assert.Equal(t, "review", newState.Name.ValueString())
	assert.Equal(t, "Code Review", newState.DisplayName.ValueString())
	assert.True(t, newState.Enabled.ValueBool())
	assert.False(t, newState.IsGithubSourced.ValueBool())
}

func TestAgentSkillResource_Update(t *testing.T) {
	r := setupTestAgentSkillResource(t)
	ctx := context.Background()

	var state AgentSkillResourceModel
	state.ID = types.StringValue("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("review")
	state.DisplayName = types.StringValue("Code Review")
	state.Description = types.StringValue("Reviews Terraform plans before apply")
	state.Enabled = types.BoolValue(true)
	state.Config = types.StringValue("")
	state.SourceRepo = types.StringValue("")
	state.SourcePath = types.StringValue("")
	state.SourceRef = types.StringValue("")
	state.IsGithubSourced = types.BoolValue(false)
	state.CreatedAt = types.StringValue("2025-07-07T12:00:00Z")

	var plan AgentSkillResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("review")
	plan.DisplayName = types.StringValue("Updated Code Review")
	plan.Description = types.StringValue("Updated description")
	plan.Enabled = types.BoolValue(false)
	plan.Config = types.StringValue(`{"model": "claude-opus-4-6"}`)
	plan.SourceRepo = types.StringValue("https://github.com/example/skills")
	plan.SourcePath = types.StringValue("skills/review")
	plan.SourceRef = types.StringValue("main")

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
	}
	diags := request.State.Set(ctx, &state)
	require.Empty(t, diags)
	diags = request.Plan.Set(ctx, &plan)
	require.Empty(t, diags)

	response := resource.UpdateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.Update(ctx, request, &response)

	if response.Diagnostics.HasError() {
		for _, diag := range response.Diagnostics.Errors() {
			t.Logf("Error: %s - %s", diag.Summary(), diag.Detail())
		}
	}
	require.False(t, response.Diagnostics.HasError())

	var newState AgentSkillResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", newState.ID.ValueString())
	assert.Equal(t, "Updated Code Review", newState.DisplayName.ValueString())
	assert.Equal(t, "Updated description", newState.Description.ValueString())
	assert.False(t, newState.Enabled.ValueBool())
	assert.True(t, newState.IsGithubSourced.ValueBool())
	assert.Equal(t, "https://github.com/example/skills", newState.SourceRepo.ValueString())
	assert.Equal(t, "skills/review", newState.SourcePath.ValueString())
	assert.Equal(t, "main", newState.SourceRef.ValueString())
}

func TestAgentSkillResource_Delete(t *testing.T) {
	r := setupTestAgentSkillResource(t)
	ctx := context.Background()

	var state AgentSkillResourceModel
	state.ID = types.StringValue("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("review")
	state.DisplayName = types.StringValue("Code Review")
	state.Description = types.StringValue("")
	state.Enabled = types.BoolValue(true)
	state.Config = types.StringValue("")
	state.SourceRepo = types.StringValue("")
	state.SourcePath = types.StringValue("")
	state.SourceRef = types.StringValue("")
	state.IsGithubSourced = types.BoolValue(false)
	state.CreatedAt = types.StringValue("2025-07-07T12:00:00Z")

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.DeleteRequest{
		State: tfsdk.State{Schema: schemaResp.Schema},
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

func TestAgentSkillResource_ImportState(t *testing.T) {
	r := setupTestAgentSkillResource(t)
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.ImportStateRequest{
		ID: "test-org:a1b2c3d4-e5f6-7890-abcd-ef1234567890",
	}
	response := resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.ImportState(ctx, request, &response)

	require.False(t, response.Diagnostics.HasError())

	var state AgentSkillResourceModel
	diags := response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", state.ID.ValueString())
	assert.Equal(t, "test-org", state.OrganizationName.ValueString())
	assert.Equal(t, "review", state.Name.ValueString())
	assert.Equal(t, "Code Review", state.DisplayName.ValueString())
	assert.True(t, state.Enabled.ValueBool())
	assert.False(t, state.IsGithubSourced.ValueBool())
}

func TestAgentSkillResource_ImportState_InvalidFormat(t *testing.T) {
	r := setupTestAgentSkillResource(t)
	ctx := context.Background()

	// Single part — missing colon separator
	request := resource.ImportStateRequest{ID: "invalid-format"}
	response := resource.ImportStateResponse{}

	r.ImportState(ctx, request, &response)

	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid import ID format")

	// Empty parts
	request.ID = ":"
	response = resource.ImportStateResponse{}
	r.ImportState(ctx, request, &response)

	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid import ID format")
}

func TestAgentSkillResource_ImportState_NotFound(t *testing.T) {
	r := setupTestAgentSkillResource(t)
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.ImportStateRequest{
		ID: "test-org:00000000-0000-0000-0000-000000000000",
	}
	response := resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.ImportState(ctx, request, &response)

	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Skill not found")
}

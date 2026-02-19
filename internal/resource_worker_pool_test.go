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

type MockWorkerPoolRoundTripper struct{}

func (m *MockWorkerPoolRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	url := req.URL.String()

	// Handle Create (POST /api/workers/{org}/pools/)
	if req.Method == http.MethodPost && strings.Contains(url, "/api/workers/test-org/pools/") {
		jsonResp := `{
			"id": "b2c3d4e5-f6a7-8901-bcde-f23456789012",
			"name": "production-pool",
			"registration_token": "tok_abc123def456",
			"workers_count": 0,
			"restrict_to_assigned": false
		}`
		resp.StatusCode = http.StatusCreated
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Read (GET /api/workers/{org}/pools/{id}/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/workers/test-org/pools/b2c3d4e5-f6a7-8901-bcde-f23456789012") {
		jsonResp := `{
			"id": "b2c3d4e5-f6a7-8901-bcde-f23456789012",
			"name": "production-pool",
			"workers_count": 3,
			"restrict_to_assigned": false
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Update (PATCH /api/workers/{org}/pools/{id}/)
	if req.Method == http.MethodPatch && strings.Contains(url, "/api/workers/test-org/pools/b2c3d4e5-f6a7-8901-bcde-f23456789012") {
		jsonResp := `{
			"id": "b2c3d4e5-f6a7-8901-bcde-f23456789012",
			"name": "updated-pool",
			"workers_count": 3,
			"restrict_to_assigned": true
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle Delete (DELETE /api/workers/{org}/pools/{id}/)
	if req.Method == http.MethodDelete && strings.Contains(url, "/api/workers/test-org/pools/b2c3d4e5-f6a7-8901-bcde-f23456789012") {
		resp.StatusCode = http.StatusNoContent
		resp.Body = io.NopCloser(strings.NewReader(""))
		return resp, nil
	}

	// Handle List (GET /api/workers/{org}/pools/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/workers/test-org/pools/") {
		jsonResp := `[{
			"id": "b2c3d4e5-f6a7-8901-bcde-f23456789012",
			"name": "production-pool",
			"workers_count": 3,
			"restrict_to_assigned": false
		}]`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

func setupTestWorkerPoolResource(t *testing.T) *WorkerPoolResource {
	t.Helper()
	httpClient := &http.Client{Transport: &MockWorkerPoolRoundTripper{}}
	provider := &InfradotsProvider{
		host:   "api.infradots.com",
		token:  "test-token",
		client: httpClient,
	}
	return &WorkerPoolResource{provider: provider}
}

func TestWorkerPoolResource_Metadata(t *testing.T) {
	r := NewWorkerPoolResource()
	ctx := context.Background()
	resp := &resource.MetadataResponse{}
	r.Metadata(ctx, resource.MetadataRequest{}, resp)
	assert.Equal(t, "infradots_worker_pool", resp.TypeName)
}

func TestWorkerPoolResource_Schema(t *testing.T) {
	r := NewWorkerPoolResource()
	ctx := context.Background()
	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)

	attrs := resp.Schema.Attributes
	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "name")
	assert.Contains(t, attrs, "registration_token")
	assert.Contains(t, attrs, "restrict_to_assigned")

	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Computed)

	orgAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgAttr.Required)

	nameAttr := attrs["name"].(schema.StringAttribute)
	assert.True(t, nameAttr.Required)

	tokenAttr := attrs["registration_token"].(schema.StringAttribute)
	assert.True(t, tokenAttr.Computed)
	assert.True(t, tokenAttr.Sensitive)
}

func TestWorkerPoolResource_Create(t *testing.T) {
	r := setupTestWorkerPoolResource(t)
	ctx := context.Background()

	var plan WorkerPoolResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("production-pool")
	plan.RestrictToAssigned = types.BoolValue(false)

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

	var state WorkerPoolResourceModel
	diags = response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "b2c3d4e5-f6a7-8901-bcde-f23456789012", state.ID.ValueString())
	assert.Equal(t, "production-pool", state.Name.ValueString())
	assert.Equal(t, "tok_abc123def456", state.RegistrationToken.ValueString())
	assert.False(t, state.RestrictToAssigned.ValueBool())
}

func TestWorkerPoolResource_Read(t *testing.T) {
	r := setupTestWorkerPoolResource(t)
	ctx := context.Background()

	var state WorkerPoolResourceModel
	state.ID = types.StringValue("b2c3d4e5-f6a7-8901-bcde-f23456789012")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("production-pool")
	state.RestrictToAssigned = types.BoolValue(false)
	state.RegistrationToken = types.StringValue("tok_abc123def456")

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

	var newState WorkerPoolResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	assert.Equal(t, "b2c3d4e5-f6a7-8901-bcde-f23456789012", newState.ID.ValueString())
	assert.Equal(t, "production-pool", newState.Name.ValueString())
	assert.False(t, newState.RestrictToAssigned.ValueBool())
}

func TestWorkerPoolResource_Update(t *testing.T) {
	r := setupTestWorkerPoolResource(t)
	ctx := context.Background()

	var state WorkerPoolResourceModel
	state.ID = types.StringValue("b2c3d4e5-f6a7-8901-bcde-f23456789012")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("production-pool")
	state.RestrictToAssigned = types.BoolValue(false)
	state.RegistrationToken = types.StringValue("tok_abc123def456")

	var plan WorkerPoolResourceModel
	plan.ID = types.StringValue("b2c3d4e5-f6a7-8901-bcde-f23456789012")
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("updated-pool")
	plan.RestrictToAssigned = types.BoolValue(true)
	plan.RegistrationToken = types.StringValue("tok_abc123def456")

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

	var newState WorkerPoolResourceModel
	diags = response.State.Get(ctx, &newState)
	require.Empty(t, diags)

	assert.Equal(t, "updated-pool", newState.Name.ValueString())
	assert.True(t, newState.RestrictToAssigned.ValueBool())
	assert.Equal(t, "tok_abc123def456", newState.RegistrationToken.ValueString())
}

func TestWorkerPoolResource_Delete(t *testing.T) {
	r := setupTestWorkerPoolResource(t)
	ctx := context.Background()

	var state WorkerPoolResourceModel
	state.ID = types.StringValue("b2c3d4e5-f6a7-8901-bcde-f23456789012")
	state.OrganizationName = types.StringValue("test-org")
	state.Name = types.StringValue("production-pool")
	state.RestrictToAssigned = types.BoolValue(false)
	state.RegistrationToken = types.StringValue("tok_abc123def456")

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

func TestWorkerPoolResource_ImportState(t *testing.T) {
	r := setupTestWorkerPoolResource(t)
	ctx := context.Background()

	request := resource.ImportStateRequest{ID: "test-org:production-pool"}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	response := resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.ImportState(ctx, request, &response)
	require.False(t, response.Diagnostics.HasError())

	var state WorkerPoolResourceModel
	diags := response.State.Get(ctx, &state)
	require.Empty(t, diags)

	assert.Equal(t, "b2c3d4e5-f6a7-8901-bcde-f23456789012", state.ID.ValueString())
	assert.Equal(t, "test-org", state.OrganizationName.ValueString())
	assert.Equal(t, "production-pool", state.Name.ValueString())
}

func TestWorkerPoolResource_ImportState_InvalidFormat(t *testing.T) {
	r := setupTestWorkerPoolResource(t)
	ctx := context.Background()

	request := resource.ImportStateRequest{ID: "invalid"}
	response := resource.ImportStateResponse{}

	r.ImportState(ctx, request, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics.Errors()[0].Summary(), "Invalid import ID format")
}

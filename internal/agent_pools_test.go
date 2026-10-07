package internal

import (
	"context"
	"encoding/json"
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

// recordingTransport answers every request with `respond` and keeps what was sent.
type recordingTransport struct {
	requests []recordedRequest
	respond  func(req *http.Request) (int, string)
}

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := recordedRequest{Method: req.Method, Path: req.URL.Path}
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &rec.Body)
		}
	}
	t.requests = append(t.requests, rec)
	status, body := t.respond(req)
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func testProvider(t *recordingTransport) *InfradotsProvider {
	return &InfradotsProvider{host: "api.infradots.com", token: "test-token", client: &http.Client{Transport: t}}
}

func TestWorkerPoolResource_CreatesAnAgentPool(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) {
		return http.StatusCreated, `{"id": "pool-1", "name": "k8s-agents", "kind": "agent", "registration_token": "tok", "restrict_to_assigned": false}`
	}}
	r := &WorkerPoolResource{provider: testProvider(transport)}
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	plan := WorkerPoolResourceModel{OrganizationName: types.StringValue("acme"), Name: types.StringValue("k8s-agents"),
		Kind: types.StringValue("agent"), RestrictToAssigned: types.BoolValue(false)}
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	assert.Equal(t, "agent", transport.requests[0].Body["kind"])
	var state WorkerPoolResourceModel
	require.Empty(t, resp.State.Get(ctx, &state))
	assert.Equal(t, "agent", state.Kind.ValueString())
	assert.Equal(t, "tok", state.RegistrationToken.ValueString())
}

func TestWorkerPoolKind_DefaultsToExecutorForOlderResponses(t *testing.T) {
	assert.Equal(t, "executor", WorkerPoolAPIResponse{}.kindOrDefault())
	assert.Equal(t, "agent", WorkerPoolAPIResponse{Kind: "agent"}.kindOrDefault())
}

func TestWorkerPoolResource_KindIsValidatedAndReplacesThePool(t *testing.T) {
	r := NewWorkerPoolResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	kind, ok := schemaResp.Schema.Attributes["kind"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, kind.IsOptional())
	assert.True(t, kind.IsComputed())
	require.Len(t, kind.Validators, 1)
	assert.Contains(t, kind.Validators[0].Description(context.Background()), "executor")
	require.Len(t, kind.PlanModifiers, 1)
	assert.Contains(t, kind.PlanModifiers[0].Description(context.Background()), "destroy and recreate")
}

func TestWorkspaceUpdateRequest_PoolsCanBeSetClearedOrLeftAlone(t *testing.T) {
	encode := func(req WorkspaceUpdateRequest) map[string]any {
		raw, err := json.Marshal(req)
		require.NoError(t, err)
		var out map[string]any
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}
	untouched := encode(WorkspaceUpdateRequest{Name: "ws"})
	assert.NotContains(t, untouched, "worker_pool")
	assert.NotContains(t, untouched, "agent_pool")

	set := encode(WorkspaceUpdateRequest{AgentPool: nullableID(types.StringValue("pool-1"))})
	assert.Equal(t, "pool-1", set["agent_pool"])

	// Removed from the configuration: sent as null, so the API unassigns it.
	for _, removed := range []types.String{types.StringNull(), types.StringValue("")} {
		cleared := encode(WorkspaceUpdateRequest{WorkerPool: nullableID(removed), AgentPool: nullableID(removed)})
		assert.Contains(t, cleared, "worker_pool")
		assert.Nil(t, cleared["worker_pool"])
		assert.Contains(t, cleared, "agent_pool")
		assert.Nil(t, cleared["agent_pool"])
	}
}

func TestWorkspaceResponse_PoolsMapToNullWhenUnassigned(t *testing.T) {
	pool := "pool-1"
	var data WorkspaceResourceModel
	mapWorkspaceResponseToModel(context.Background(), &data, WorkspaceAPIResponse{AgentPool: &pool})
	assert.Equal(t, "pool-1", data.AgentPoolID.ValueString())
	assert.True(t, data.WorkerPoolID.IsNull())
}

func orgAgentPoolResource(t *testing.T, respond func(*http.Request) (int, string)) (*OrganizationAgentPoolResource, *recordingTransport, *resource.SchemaResponse) {
	t.Helper()
	transport := &recordingTransport{respond: respond}
	r := &OrganizationAgentPoolResource{provider: testProvider(transport)}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return r, transport, schemaResp
}

func TestOrganizationAgentPool_SetsTheOrganizationsPool(t *testing.T) {
	r, transport, schemaResp := orgAgentPoolResource(t, func(*http.Request) (int, string) {
		return http.StatusOK, `{"id": "org-uuid", "name": "acme", "agent_pool": "pool-1"}`
	})
	ctx := context.Background()
	plan := OrganizationAgentPoolResourceModel{OrganizationName: types.StringValue("acme"), AgentPoolID: types.StringValue("pool-1")}
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	require.Len(t, transport.requests, 1)
	assert.Equal(t, http.MethodPatch, transport.requests[0].Method)
	assert.Equal(t, "/api/organizations/acme/", transport.requests[0].Path)
	assert.Equal(t, map[string]any{"agent_pool": "pool-1"}, transport.requests[0].Body)
	var state OrganizationAgentPoolResourceModel
	require.Empty(t, resp.State.Get(ctx, &state))
	assert.Equal(t, "acme", state.ID.ValueString())
	assert.Equal(t, "pool-1", state.AgentPoolID.ValueString())
}

func TestOrganizationAgentPool_ClearedOutsideTerraformLeavesState(t *testing.T) {
	r, _, schemaResp := orgAgentPoolResource(t, func(*http.Request) (int, string) {
		return http.StatusOK, `{"id": "org-uuid", "name": "acme", "agent_pool": null}`
	})
	ctx := context.Background()
	state := OrganizationAgentPoolResourceModel{ID: types.StringValue("acme"), OrganizationName: types.StringValue("acme"),
		AgentPoolID: types.StringValue("pool-1")}
	req := resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	require.Empty(t, req.State.Set(ctx, &state))
	resp := resource.ReadResponse{State: req.State}
	r.Read(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a cleared pool is removed from state so the next plan sets it again")
}

func TestOrganizationAgentPool_DestroyClearsThePool(t *testing.T) {
	r, transport, schemaResp := orgAgentPoolResource(t, func(*http.Request) (int, string) {
		return http.StatusOK, `{"id": "org-uuid", "name": "acme", "agent_pool": null}`
	})
	ctx := context.Background()
	state := OrganizationAgentPoolResourceModel{ID: types.StringValue("acme"), OrganizationName: types.StringValue("acme"),
		AgentPoolID: types.StringValue("pool-1")}
	req := resource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	require.Empty(t, req.State.Set(ctx, &state))
	resp := resource.DeleteResponse{State: req.State}
	r.Delete(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	require.Len(t, transport.requests, 1)
	assert.Equal(t, http.MethodPatch, transport.requests[0].Method)
	assert.Contains(t, transport.requests[0].Body, "agent_pool")
	assert.Nil(t, transport.requests[0].Body["agent_pool"])
}

func TestOrganizationAgentPool_ReportsARefusal(t *testing.T) {
	r, _, schemaResp := orgAgentPoolResource(t, func(*http.Request) (int, string) {
		return http.StatusBadRequest, `{"agent_pool": ["Not an agent pool."]}`
	})
	ctx := context.Background()
	plan := OrganizationAgentPoolResourceModel{OrganizationName: types.StringValue("acme"), AgentPoolID: types.StringValue("executor-pool")}
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, req, &resp)
	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "Not an agent pool.")
}

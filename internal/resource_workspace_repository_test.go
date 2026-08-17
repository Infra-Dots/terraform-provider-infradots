package internal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRoundTripper records the request body so tests can assert on what the provider actually
// sends, and returns a configurable "repository" block on create. The shared
// MockWorkspaceRoundTripper answers with a fixed payload and discards the request.
type captureRoundTripper struct {
	lastBody   map[string]any
	lastMethod string
	// repositoryJSON is spliced into the create response; empty means the API sent no
	// "repository" key at all (a workspace with no VCS).
	repositoryJSON string
}

func (m *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastMethod = req.Method
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		m.lastBody = map[string]any{}
		_ = json.Unmarshal(raw, &m.lastBody)
	}

	body := `{
		"id": "3f340e3c-89f1-4321-bcde-eff34567890a",
		"name": "test-workspace",
		"description": "",
		"source": "acme/infra",
		"branch": "main",
		"terraform_version": "1.5.0",
		"created_at": "2025-07-07T12:00:00Z",
		"updated_at": "2025-07-07T12:00:00Z",
		"locked": false,
		"auto_apply": false,
		"iac_type": "TF",
		"default_job_action": "plan",
		"worker_pool": null,
		"folder": "/",
		"execution_mode": "Remote",
		"tags": {},
		"agents_enabled": false,
		"vcs": null`
	if m.repositoryJSON != "" {
		body += ",\n\t\t\"repository\": " + m.repositoryJSON
	}
	body += "\n\t}"

	status := http.StatusOK
	if req.Method == http.MethodPost {
		status = http.StatusCreated
	}
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func setupCapturingWorkspaceResource(t *testing.T, repositoryJSON string) (*WorkspaceResource, *captureRoundTripper) {
	t.Helper()
	transport := &captureRoundTripper{repositoryJSON: repositoryJSON}
	return &WorkspaceResource{
		provider: &InfradotsProvider{
			host:   "api.infradots.com",
			token:  "test-token",
			client: &http.Client{Transport: transport},
		},
	}, transport
}

// basePlan is the minimum viable workspace plan; individual tests layer their fields on top.
func basePlan() WorkspaceResourceModel {
	var plan WorkspaceResourceModel
	plan.OrganizationName = types.StringValue("test-org")
	plan.Name = types.StringValue("test-workspace")
	plan.Source = types.StringValue("acme/infra")
	plan.Branch = types.StringValue("main")
	plan.TerraformVersion = types.StringValue("1.5.0")
	plan.Locked = types.BoolValue(false)
	plan.AutoApply = types.BoolValue(false)
	plan.IacType = types.StringValue("TF")
	plan.DefaultJobAction = types.StringValue("plan")
	plan.Folder = types.StringValue("/")
	plan.ExecutionMode = types.StringValue("Remote")
	plan.AgentsEnabled = types.BoolValue(false)
	plan.Tags = types.MapValueMust(types.StringType, map[string]attr.Value{})
	plan.TflintPlugins = types.ListNull(types.StringType)
	plan.TriggerPatterns = types.ListNull(triggerPatternObjectType)
	plan.VCS = types.ObjectNull(map[string]attr.Type{
		"id":          types.StringType,
		"name":        types.StringType,
		"vcs_type":    types.StringType,
		"url":         types.StringType,
		"description": types.StringType,
		"created_at":  types.StringType,
		"updated_at":  types.StringType,
	})
	return plan
}

func runCreate(t *testing.T, r *WorkspaceResource, plan WorkspaceResourceModel) resource.CreateResponse {
	t.Helper()
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	request := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	require.Empty(t, request.Plan.Set(ctx, &plan))

	response := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, request, &response)
	return response
}

// TestWorkspaceCreateSendsVcsId guards a bug where vcs_id was in the schema and the docs but
// never marshalled, so the provider could not attach a VCS to a workspace at all.
func TestWorkspaceCreateSendsVcsId(t *testing.T) {
	r, transport := setupCapturingWorkspaceResource(t, "")
	plan := basePlan()
	plan.VcsId = types.StringValue("vcs-12345")

	response := runCreate(t, r, plan)
	require.False(t, response.Diagnostics.HasError())

	assert.Equal(t, "vcs-12345", transport.lastBody["vcs"],
		"the backend reads the VCS from the 'vcs' key on the create body")
}

func TestWorkspaceCreateOmitsVcsWhenUnset(t *testing.T) {
	r, transport := setupCapturingWorkspaceResource(t, "")
	response := runCreate(t, r, basePlan())
	require.False(t, response.Diagnostics.HasError())

	_, present := transport.lastBody["vcs"]
	assert.False(t, present, "a workspace with no VCS must not send an empty vcs id")
}

func TestWorkspaceCreateSendsCreateRepository(t *testing.T) {
	r, transport := setupCapturingWorkspaceResource(t, `{"source": "acme/infra", "exists": true, "created": true}`)
	plan := basePlan()
	plan.VcsId = types.StringValue("vcs-12345")
	plan.CreateRepository = types.BoolValue(true)

	response := runCreate(t, r, plan)
	require.False(t, response.Diagnostics.HasError())

	assert.Equal(t, true, transport.lastBody["create_repository"])
}

func TestWorkspaceCreateOmitsCreateRepositoryWhenUnset(t *testing.T) {
	r, transport := setupCapturingWorkspaceResource(t, "")
	response := runCreate(t, r, basePlan())
	require.False(t, response.Diagnostics.HasError())

	_, present := transport.lastBody["create_repository"]
	assert.False(t, present, "an unset optional must not be sent, so the API keeps its default")
}

func TestWorkspaceCreateRecordsRepositoryCreated(t *testing.T) {
	r, _ := setupCapturingWorkspaceResource(t, `{"source": "acme/infra", "exists": true, "created": true}`)
	plan := basePlan()
	plan.VcsId = types.StringValue("vcs-12345")
	plan.CreateRepository = types.BoolValue(true)

	response := runCreate(t, r, plan)
	require.False(t, response.Diagnostics.HasError())

	var state WorkspaceResourceModel
	require.Empty(t, response.State.Get(context.Background(), &state))
	assert.True(t, state.RepositoryCreated.ValueBool())
	assert.Empty(t, response.Diagnostics.Warnings())
}

// TestWorkspaceCreateWarnsOnMissingRepository: a missing repo is deliberately not an error --
// the workspace is created either way -- but it must not pass silently, because the failure
// would otherwise only surface as a clone error on the first job.
func TestWorkspaceCreateWarnsOnMissingRepository(t *testing.T) {
	r, _ := setupCapturingWorkspaceResource(t,
		`{"source": "acme/infra", "exists": false, "created": false, "warning": "Repository acme/infra was not found"}`)
	plan := basePlan()
	plan.VcsId = types.StringValue("vcs-12345")

	response := runCreate(t, r, plan)
	require.False(t, response.Diagnostics.HasError())

	warnings := response.Diagnostics.Warnings()
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Detail(), "was not found")

	var state WorkspaceResourceModel
	require.Empty(t, response.State.Get(context.Background(), &state))
	assert.False(t, state.RepositoryCreated.ValueBool())
}

func TestWorkspaceCreateWithoutRepositoryBlockIsKnownFalse(t *testing.T) {
	r, _ := setupCapturingWorkspaceResource(t, "")
	response := runCreate(t, r, basePlan())
	require.False(t, response.Diagnostics.HasError())

	var state WorkspaceResourceModel
	require.Empty(t, response.State.Get(context.Background(), &state))
	assert.False(t, state.RepositoryCreated.IsNull(),
		"repository_created is computed, so it must hold a known value even with no VCS")
	assert.False(t, state.RepositoryCreated.ValueBool())
	assert.Empty(t, response.Diagnostics.Warnings())
}

// TestWorkspaceReadLeavesRepositoryCreatedAlone: the read endpoint never reports repository
// state, so refreshing it from the response would null a computed attribute Create had set --
// the classic "Provider produced inconsistent result after apply".
func TestWorkspaceReadLeavesRepositoryCreatedAlone(t *testing.T) {
	r, _ := setupCapturingWorkspaceResource(t, "")
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	prior := basePlan()
	prior.ID = types.StringValue("3f340e3c-89f1-4321-bcde-eff34567890a")
	prior.RepositoryCreated = types.BoolValue(true)

	request := resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	require.Empty(t, request.State.Set(ctx, &prior))

	response := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Read(ctx, request, &response)
	require.False(t, response.Diagnostics.HasError())

	var state WorkspaceResourceModel
	require.Empty(t, response.State.Get(ctx, &state))
	assert.True(t, state.RepositoryCreated.ValueBool())
}

func TestWorkspaceUpdateSendsVcsIdWhenChanged(t *testing.T) {
	r, transport := setupCapturingWorkspaceResource(t, "")
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	state := basePlan()
	state.ID = types.StringValue("3f340e3c-89f1-4321-bcde-eff34567890a")
	state.RepositoryCreated = types.BoolValue(false)

	plan := state
	plan.VcsId = types.StringValue("vcs-99999")

	request := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	require.Empty(t, request.Plan.Set(ctx, &plan))
	require.Empty(t, request.State.Set(ctx, &state))

	response := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Update(ctx, request, &response)
	require.False(t, response.Diagnostics.HasError())

	assert.Equal(t, "vcs-99999", transport.lastBody["vcs"])
}

func TestWorkspaceSchemaRepositoryAttributes(t *testing.T) {
	r := &WorkspaceResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	createRepo, ok := schemaResp.Schema.Attributes["create_repository"]
	require.True(t, ok)
	assert.True(t, createRepo.IsOptional())
	assert.False(t, createRepo.IsComputed(), "an opt-in flag the practitioner sets, never derived")

	created, ok := schemaResp.Schema.Attributes["repository_created"]
	require.True(t, ok)
	assert.True(t, created.IsComputed())
	assert.False(t, created.IsOptional(), "reported by the API, not configurable")
}

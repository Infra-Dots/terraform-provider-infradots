package internal

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMapWorkspaceIntegrationKeepsRunAfterStage guards the regression that broke the integration
// tests with:
//
//	Error: Provider produced inconsistent result after apply
//	.run_after_stage: was cty.StringVal("apply"), but now cty.StringVal("")
//
// run_after_stage is Computed with a default, so Terraform requires it to hold a known value
// after apply. The API stopped returning it when integrations moved to named events, so mapping
// it straight off the response (data.RunAfterStage = types.StringValue(wi.RunAfterStage)) wrote
// an empty string over the planned "apply". A deprecated attribute the API no longer echoes has
// to be carried from configuration/state, never refreshed from the response.
func TestMapWorkspaceIntegrationKeepsRunAfterStage(t *testing.T) {
	response := WorkspaceIntegrationAPIResponse{
		ID:          "wi-1",
		Integration: WorkspaceIntegrationRef{ID: "int-1", Name: "slack"},
		Events:      []string{"run.applied"},
	}

	t.Run("keeps the configured value", func(t *testing.T) {
		data := WorkspaceIntegrationResourceModel{RunAfterStage: types.StringValue("apply")}
		mapWorkspaceIntegrationToModel(context.Background(), &data, response)
		assert.Equal(t, "apply", data.RunAfterStage.ValueString())
	})

	t.Run("keeps a non-default configured value", func(t *testing.T) {
		data := WorkspaceIntegrationResourceModel{RunAfterStage: types.StringValue("plan")}
		mapWorkspaceIntegrationToModel(context.Background(), &data, response)
		assert.Equal(t, "plan", data.RunAfterStage.ValueString())
	})

	t.Run("falls back to the default when unset, e.g. on import", func(t *testing.T) {
		data := WorkspaceIntegrationResourceModel{RunAfterStage: types.StringNull()}
		mapWorkspaceIntegrationToModel(context.Background(), &data, response)
		assert.Equal(t, "apply", data.RunAfterStage.ValueString())
		assert.False(t, data.RunAfterStage.IsNull(),
			"a computed attribute must hold a known value after apply")
	})

	t.Run("resolves unknown rather than leaving it in state", func(t *testing.T) {
		data := WorkspaceIntegrationResourceModel{RunAfterStage: types.StringUnknown()}
		mapWorkspaceIntegrationToModel(context.Background(), &data, response)
		assert.False(t, data.RunAfterStage.IsUnknown())
		assert.Equal(t, "apply", data.RunAfterStage.ValueString())
	})
}

func TestMapWorkspaceIntegrationEvents(t *testing.T) {
	ctx := context.Background()

	t.Run("maps the events the API reports", func(t *testing.T) {
		var data WorkspaceIntegrationResourceModel
		mapWorkspaceIntegrationToModel(ctx, &data, WorkspaceIntegrationAPIResponse{
			Integration: WorkspaceIntegrationRef{ID: "int-1"},
			Events:      []string{"run.applied", "agent.review_completed"},
		})

		var events []string
		require.Empty(t, data.Events.ElementsAs(ctx, &events, false))
		assert.Equal(t, []string{"run.applied", "agent.review_completed"}, events)
	})

	t.Run("absent events become an empty list, not null", func(t *testing.T) {
		// events is Computed, so null after apply would be another inconsistent-result error.
		var data WorkspaceIntegrationResourceModel
		mapWorkspaceIntegrationToModel(ctx, &data, WorkspaceIntegrationAPIResponse{
			Integration: WorkspaceIntegrationRef{ID: "int-1"},
		})
		assert.False(t, data.Events.IsNull())
		assert.Equal(t, 0, len(data.Events.Elements()))
	})
}

func TestWorkspaceIntegrationSchemaMarksRunAfterStageDeprecated(t *testing.T) {
	r := &WorkspaceIntegrationResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	attr, ok := schemaResp.Schema.Attributes["run_after_stage"]
	require.True(t, ok)
	assert.NotEmpty(t, attr.GetDeprecationMessage(),
		"still accepted so existing configs keep working, but it no longer does anything")
	assert.True(t, attr.IsComputed(), "which is why it must be carried, not refreshed")
}

// events = [] mutes an integration. omitempty used to drop the empty list, the attach endpoint
// then applied the org defaults, and apply failed with ".events: new element 0 has appeared".
func TestWorkspaceIntegrationCreate_SendsEvents(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		events  types.List
		wantKey bool
	}{
		"empty list is sent as []":     {types.ListValueMust(types.StringType, []attr.Value{}), true},
		"null is omitted for defaults": {types.ListNull(types.StringType), false},
	} {
		t.Run(name, func(t *testing.T) {
			transport := &recordingTransport{respond: func(*http.Request) (int, string) {
				return http.StatusCreated, `{"id": "wi-1", "integration": {"id": "int-1"}, "events": []}`
			}}
			r := &WorkspaceIntegrationResource{provider: testProvider(transport)}
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

			plan := WorkspaceIntegrationResourceModel{
				OrganizationName: types.StringValue("acme"), WorkspaceName: types.StringValue("ws"),
				IntegrationID: types.StringValue("int-1"), RunAfterStage: types.StringValue("apply"),
				Events: tc.events, SlackChannels: types.ListNull(types.StringType),
				SlackEnvChannels: types.MapNull(types.StringType),
			}
			req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
			require.Empty(t, req.Plan.Set(ctx, &plan))
			resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Create(ctx, req, &resp)
			require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

			events, ok := transport.requests[0].Body["events"]
			assert.Equal(t, tc.wantKey, ok)
			if tc.wantKey {
				assert.Equal(t, []any{}, events)
			}
		})
	}
}

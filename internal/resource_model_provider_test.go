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

// What the API returns for a bedrock provider: the plaintext settings under `config`, never the
// secret ones.
const bedrockResponse = `{
  "id": "mp-1", "name": "our-bedrock", "provider": "bedrock",
  "config": {"aws_region": "eu-west-1"},
  "model_map": {"claude-opus-5": "eu.anthropic.claude-opus-5-v1:0"},
  "enabled": true, "verified_at": null, "verification_error": "",
  "description": "", "created_at": "2026-10-22T10:00:00Z", "updated_at": "2026-10-22T10:00:00Z"
}`

func stringMapValue(pairs map[string]string) types.Map {
	elements := map[string]attr.Value{}
	for key, value := range pairs {
		elements[key] = types.StringValue(value)
	}
	return types.MapValueMust(types.StringType, elements)
}

func modelProviderPlan() ModelProviderResourceModel {
	return ModelProviderResourceModel{
		OrganizationName: types.StringValue("acme"),
		Name:             types.StringValue("our-bedrock"),
		Provider:         types.StringValue("bedrock"),
		APIKey:           types.StringNull(),
		Settings: stringMapValue(map[string]string{
			"aws_region": "eu-west-1", "aws_access_key": "AK", "aws_secret_key": "SK",
		}),
		ModelMap:    stringMapValue(map[string]string{"claude-opus-5": "eu.anthropic.claude-opus-5-v1:0"}),
		Enabled:     types.BoolValue(true),
		Description: types.StringValue(""),
		// Computed, so a plan carries them as unknown rather than as a zero value.
		ID:                types.StringUnknown(),
		Config:            types.MapUnknown(types.StringType),
		VerifiedAt:        types.StringUnknown(),
		VerificationError: types.StringUnknown(),
		CreatedAt:         types.StringUnknown(),
		UpdatedAt:         types.StringUnknown(),
	}
}

func TestModelProviderResource_SendsEverySettingAndKeepsTheSecretsInState(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) {
		return http.StatusCreated, bedrockResponse
	}}
	r := &ModelProviderResource{provider: testProvider(transport)}
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	plan := modelProviderPlan()
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	sent := transport.requests[0].Body["settings"].(map[string]any)
	assert.Equal(t, "eu-west-1", sent["aws_region"])
	assert.Equal(t, "SK", sent["aws_secret_key"])
	assert.Equal(t, map[string]any{"claude-opus-5": "eu.anthropic.claude-opus-5-v1:0"},
		transport.requests[0].Body["model_map"])

	var state ModelProviderResourceModel
	require.Empty(t, resp.State.Get(ctx, &state))
	assert.Equal(t, "mp-1", state.ID.ValueString())
	// The configured secrets survive the response, which could not return them.
	assert.Equal(t, plan.Settings, state.Settings)
	// And the plaintext half reads back, so a region changed in the UI shows up as drift.
	assert.Equal(t, stringMapValue(map[string]string{"aws_region": "eu-west-1"}), state.Config)
}

// The write-only-secrets rule: a read must never replace the configured settings with what the API
// returned, because the API returns none of them. Overwriting here is what produces "inconsistent
// values for sensitive attribute" on the next apply.
func TestModelProviderResource_ReadDoesNotClearTheConfiguredSecrets(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) {
		return http.StatusOK, bedrockResponse
	}}
	r := &ModelProviderResource{provider: testProvider(transport)}
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	stored := modelProviderPlan()
	stored.ID = types.StringValue("mp-1")
	stored.Config = stringMapValue(map[string]string{"aws_region": "eu-west-1"})
	stored.VerifiedAt = types.StringNull()
	stored.VerificationError = types.StringValue("")
	stored.CreatedAt = types.StringValue("2026-10-22T10:00:00Z")
	stored.UpdatedAt = types.StringValue("2026-10-22T10:00:00Z")

	req := resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	require.Empty(t, req.State.Set(ctx, &stored))
	resp := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Read(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var state ModelProviderResourceModel
	require.Empty(t, resp.State.Get(ctx, &state))
	assert.Equal(t, stored.Settings, state.Settings)
	assert.True(t, state.APIKey.IsNull())
}

// An edit that touches only the name must not resend the settings: writing them replaces them
// wholesale and drops the provider's verification.
func TestModelProviderResource_UpdateOnlySendsWhatChanged(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) {
		return http.StatusOK, bedrockResponse
	}}
	r := &ModelProviderResource{provider: testProvider(transport)}
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	state := modelProviderPlan()
	state.ID = types.StringValue("mp-1")
	plan := modelProviderPlan()
	plan.ID = types.StringValue("mp-1")
	plan.Description = types.StringValue("our own AWS account")

	req := resource.UpdateRequest{
		State: tfsdk.State{Schema: schemaResp.Schema},
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema},
	}
	require.Empty(t, req.State.Set(ctx, &state))
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Update(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	body := transport.requests[0].Body
	assert.Equal(t, "our own AWS account", body["description"])
	assert.NotContains(t, body, "settings")
	assert.NotContains(t, body, "model_map")
}

func TestOrganizationModelProviderResource_PointsAnOrganizationAtItsProvider(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) {
		return http.StatusOK, `{"id": "org-1", "name": "acme", "model_provider": "mp-1", "allow_platform_fallback": false}`
	}}
	r := &OrganizationModelProviderResource{provider: testProvider(transport)}
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	plan := OrganizationModelProviderResourceModel{
		OrganizationName:      types.StringValue("acme"),
		ModelProviderID:       types.StringValue("mp-1"),
		AllowPlatformFallback: types.BoolValue(false),
	}
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	assert.Equal(t, "mp-1", transport.requests[0].Body["model_provider"])
	assert.Equal(t, false, transport.requests[0].Body["allow_platform_fallback"])

	var state OrganizationModelProviderResourceModel
	require.Empty(t, resp.State.Get(ctx, &state))
	assert.Equal(t, "acme", state.ID.ValueString())
	assert.Equal(t, "mp-1", state.ModelProviderID.ValueString())
}

// Destroying it sends the organization's model calls back to InfraDots' own account.
func TestOrganizationModelProviderResource_DeleteClearsTheProvider(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) {
		return http.StatusOK, `{"id": "org-1", "name": "acme", "model_provider": null, "allow_platform_fallback": false}`
	}}
	r := &OrganizationModelProviderResource{provider: testProvider(transport)}
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	stored := OrganizationModelProviderResourceModel{
		ID:                    types.StringValue("acme"),
		OrganizationName:      types.StringValue("acme"),
		ModelProviderID:       types.StringValue("mp-1"),
		AllowPlatformFallback: types.BoolValue(false),
	}
	req := resource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	require.Empty(t, req.State.Set(ctx, &stored))
	resp := resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Delete(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	assert.Equal(t, http.MethodPatch, transport.requests[0].Method)
	assert.Nil(t, transport.requests[0].Body["model_provider"])
}

// A Read that finds the provider cleared outside Terraform drops the resource rather than fighting it.
func TestOrganizationModelProviderResource_ReadForgetsAClearedProvider(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) {
		return http.StatusOK, `{"id": "org-1", "name": "acme", "model_provider": null, "allow_platform_fallback": false}`
	}}
	r := &OrganizationModelProviderResource{provider: testProvider(transport)}
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	stored := OrganizationModelProviderResourceModel{
		ID:                    types.StringValue("acme"),
		OrganizationName:      types.StringValue("acme"),
		ModelProviderID:       types.StringValue("mp-1"),
		AllowPlatformFallback: types.BoolValue(false),
	}
	req := resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema}}
	require.Empty(t, req.State.Set(ctx, &stored))
	resp := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Read(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull())
}

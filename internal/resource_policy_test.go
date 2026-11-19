package internal

import (
	"context"
	"math/big"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func policySchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	resp := resource.SchemaResponse{}
	NewPolicyResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp
}

// policyModel is a policy with every setting null, as a configuration that sets none of them would be.
func policyModel(kind string) PolicyResourceModel {
	return PolicyResourceModel{
		ID: types.StringUnknown(), OrganizationName: types.StringValue("acme"), Name: types.StringValue("p"),
		Description: types.StringValue(""), Kind: types.StringValue(kind), Enforcement: types.StringValue("blocking"),
		WorkspaceTags: types.MapValueMust(types.StringType, map[string]attr.Value{"env": types.StringValue("prod")}),
		ResourceType:  types.StringNull(), Path: types.StringNull(), Op: types.StringNull(), Value: types.DynamicNull(),
		Values: types.ListNull(types.StringType), RequiredTags: types.ListNull(types.StringType),
		Ports: types.SetNull(types.Int64Type),
	}
}

func createPolicy(t *testing.T, plan PolicyResourceModel, response string) (recordedRequest, PolicyResourceModel) {
	t.Helper()
	transport := &recordingTransport{respond: func(*http.Request) (int, string) { return http.StatusCreated, response }}
	r := &PolicyResource{provider: testProvider(transport)}
	ctx := context.Background()
	s := policySchema(t)
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	var state PolicyResourceModel
	require.Empty(t, resp.State.Get(ctx, &state))
	return transport.requests[0], state
}

func TestPolicyResource_AnEqNumberStaysANumber(t *testing.T) {
	plan := policyModel("attribute")
	plan.ResourceType, plan.Path, plan.Op = types.StringValue("aws_db_instance"), types.StringValue("backup_retention_period"), types.StringValue("eq")
	plan.Value = types.DynamicValue(types.NumberValue(big.NewFloat(7)))

	sent, state := createPolicy(t, plan, `{"id": "pol-1", "name": "p", "description": "", "kind": "attribute",
		"enforcement": "blocking", "workspace_tags": {"env": "prod"},
		"config": {"resource_type": "aws_db_instance", "path": "backup_retention_period", "op": "eq", "value": 7}}`)

	assert.Equal(t, "/api/organizations/acme/policies/", sent.Path)
	assert.Equal(t, "attribute", sent.Body["kind"])
	assert.Equal(t, map[string]any{"resource_type": "aws_db_instance", "path": "backup_retention_period",
		"op": "eq", "value": float64(7)}, sent.Body["config"]) // a JSON number, not "7"
	assert.Equal(t, map[string]any{"env": "prod"}, sent.Body["workspace_tags"])
	assert.Equal(t, "pol-1", state.ID.ValueString())
	assert.True(t, state.Value.Equal(plan.Value), state.Value)
	assert.True(t, state.Values.IsNull())
}

func TestPolicyResource_InSendsItsValuesAsTheConfigValue(t *testing.T) {
	plan := policyModel("attribute")
	plan.ResourceType, plan.Path, plan.Op = types.StringValue("aws_s3_bucket"), types.StringValue("acl"), types.StringValue("not_in")
	plan.Values = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("public-read")})

	sent, state := createPolicy(t, plan, `{"id": "pol-1", "name": "p", "description": "", "kind": "attribute",
		"enforcement": "blocking", "workspace_tags": {"env": "prod"},
		"config": {"resource_type": "aws_s3_bucket", "path": "acl", "op": "not_in", "value": ["public-read"]}}`)

	assert.Equal(t, []any{"public-read"}, sent.Body["config"].(map[string]any)["value"])
	assert.True(t, state.Values.Equal(plan.Values))
	assert.True(t, state.Value.IsNull())
}

func TestPolicyResource_RequireTagsAndPortsMapToTheirConfigKeys(t *testing.T) {
	tags := policyModel("require_tags")
	tags.RequiredTags = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("owner")})
	sent, state := createPolicy(t, tags, `{"id": "pol-1", "name": "p", "description": "", "kind": "require_tags",
		"enforcement": "blocking", "workspace_tags": {"env": "prod"}, "config": {"tags": ["owner"]}}`)
	assert.Equal(t, map[string]any{"tags": []any{"owner"}}, sent.Body["config"])
	assert.True(t, state.RequiredTags.Equal(tags.RequiredTags))

	ingress := policyModel("no_open_ingress")
	ingress.Ports = types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(3389), types.Int64Value(22)})
	// The API sorts the ports; a set doesn't care.
	sent, state = createPolicy(t, ingress, `{"id": "pol-2", "name": "p", "description": "", "kind": "no_open_ingress",
		"enforcement": "blocking", "workspace_tags": {"env": "prod"}, "config": {"ports": [22, 3389]}}`)
	assert.ElementsMatch(t, []any{float64(22), float64(3389)}, sent.Body["config"].(map[string]any)["ports"])
	assert.True(t, state.Ports.Equal(ingress.Ports))
}

func TestPolicyResource_UpdateNeverSendsTheKind(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) {
		return http.StatusOK, `{"id": "pol-1", "name": "p", "description": "", "kind": "no_delete",
			"enforcement": "advisory", "workspace_tags": {}, "config": {}}`
	}}
	r := &PolicyResource{provider: testProvider(transport)}
	ctx := context.Background()
	s := policySchema(t)
	state := policyModel("no_delete")
	state.ID = types.StringValue("pol-1")
	plan := state
	plan.Enforcement = types.StringValue("advisory")
	plan.WorkspaceTags = types.MapValueMust(types.StringType, map[string]attr.Value{})
	req := resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s.Schema}, State: tfsdk.State{Schema: s.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	require.Empty(t, req.State.Set(ctx, &state))
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Update(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	sent := transport.requests[0]
	assert.Equal(t, http.MethodPatch, sent.Method)
	assert.Equal(t, "/api/organizations/acme/policies/pol-1/", sent.Path)
	assert.NotContains(t, sent.Body, "kind")
	assert.Equal(t, "advisory", sent.Body["enforcement"])
	assert.Equal(t, map[string]any{}, sent.Body["workspace_tags"]) // clearing the scope is sent, not omitted
}

func TestPolicyResource_ReadForgetsADeletedPolicy(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) { return http.StatusNotFound, `{}` }}
	r := &PolicyResource{provider: testProvider(transport)}
	ctx := context.Background()
	s := policySchema(t)
	model := policyModel("no_delete")
	model.ID = types.StringValue("pol-1")
	state := tfsdk.State{Schema: s.Schema}
	require.Empty(t, state.Set(ctx, &model))
	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull())
}

func validatePolicy(t *testing.T, model PolicyResourceModel) []string {
	t.Helper()
	ctx := context.Background()
	s := policySchema(t)
	config := tfsdk.Config{Schema: s.Schema}
	plan := tfsdk.Plan{Schema: s.Schema}
	require.Empty(t, plan.Set(ctx, &model))
	config.Raw = plan.Raw
	resp := resource.ValidateConfigResponse{}
	(&PolicyResource{}).ValidateConfig(ctx, resource.ValidateConfigRequest{Config: config}, &resp)
	var summaries []string
	for _, d := range resp.Diagnostics.Errors() {
		summaries = append(summaries, d.Summary()+": "+d.Detail())
	}
	return summaries
}

func TestPolicyResource_ValidateConfig(t *testing.T) {
	assert.Empty(t, validatePolicy(t, policyModel("no_delete")))

	foreign := policyModel("no_delete")
	foreign.Ports = types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(22)})
	assert.Equal(t, []string{"Setting not used by this kind: ports is not a setting of the no_delete kind."},
		validatePolicy(t, foreign))

	assert.Equal(t, []string{"Missing setting: The require_tags kind needs required_tags."},
		validatePolicy(t, policyModel("require_tags")))

	eq := policyModel("attribute")
	eq.ResourceType, eq.Path, eq.Op = types.StringValue("*"), types.StringValue("region"), types.StringValue("eq")
	assert.Equal(t, []string{"Missing setting: The attribute kind needs value."}, validatePolicy(t, eq))

	exists := eq
	exists.Op = types.StringValue("exists")
	exists.Values = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("x")})
	assert.Equal(t, []string{"Setting not used by this op: values is only read by op = in or not_in."},
		validatePolicy(t, exists))
}

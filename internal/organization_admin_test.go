package internal

import (
	"context"
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

// adminAPI is a fake idp holding one team and one member, recording every write it receives.
type adminAPI struct {
	teamAdmin bool
	userAdmin bool
	writes    []string // "METHOD path body"
}

func (a *adminAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}
	path := req.URL.Path
	if req.Method != http.MethodGet {
		a.writes = append(a.writes, req.Method+" "+path+" "+body)
	}
	status, out := http.StatusOK, ""
	switch {
	case req.Method == http.MethodPost && path == "/api/organizations/acme/teams/":
		a.teamAdmin = strings.Contains(body, `"organization_admin":true`)
		status, out = http.StatusCreated, a.team()
	case req.Method == http.MethodPatch && path == "/api/organizations/acme/teams/t1/":
		if strings.Contains(body, `"organization_admin"`) {
			a.teamAdmin = strings.Contains(body, `"organization_admin":true`)
		}
	case req.Method == http.MethodGet && path == "/api/organizations/acme/teams/t1/":
		out = a.team()
	case req.Method == http.MethodPost && path == "/api/users/acme/users/":
		out = `{"message": "ok"}`
	case req.Method == http.MethodPatch && path == "/api/users/acme/users/dev@acme.com/":
		a.userAdmin = strings.Contains(body, `"organization_admin":true`)
		status = http.StatusCreated
	case req.Method == http.MethodGet && path == "/api/users/acme/users/":
		out = `[{"id": "u1", "email": "dev@acme.com", "teams": [], "permissions": [], "last_login": null,
		         "organization_admin": ` + boolJSON(a.userAdmin) + `}]`
	default:
		status, out = http.StatusNotFound, `{"error": "Not found"}`
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Request: req,
		Body: io.NopCloser(strings.NewReader(out))}, nil
}

func (a *adminAPI) team() string {
	return `{"id": "t1", "name": "ops", "members": [], "permissions": [], "organization_admin": ` +
		boolJSON(a.teamAdmin) + `}`
}

func boolJSON(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func adminProvider(api *adminAPI) *InfradotsProvider {
	return &InfradotsProvider{host: "api.infradots.com", token: "t", client: &http.Client{Transport: api}}
}

func teamModel(name string, admin types.Bool) TeamResourceModel {
	return TeamResourceModel{ID: types.StringValue("t1"), OrganizationName: types.StringValue("acme"),
		Name: types.StringValue(name), Members: types.ListValueMust(types.StringType, []attr.Value{}),
		OrganizationAdmin: admin}
}

func updateTeam(t *testing.T, api *adminAPI, state, plan TeamResourceModel) TeamResourceModel {
	t.Helper()
	ctx := context.Background()
	r := &TeamResource{provider: adminProvider(api)}
	s := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, s)
	req := resource.UpdateRequest{State: tfsdk.State{Schema: s.Schema}, Plan: tfsdk.Plan{Schema: s.Schema}}
	require.Empty(t, req.State.Set(ctx, &state))
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Update(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	var out TeamResourceModel
	require.Empty(t, resp.State.Get(ctx, &out))
	return out
}

func TestTeamRenameSendsOnlyTheName(t *testing.T) {
	api := &adminAPI{teamAdmin: true}
	out := updateTeam(t, api, teamModel("ops", types.BoolValue(true)), teamModel("platform", types.BoolValue(true)))
	assert.Equal(t, []string{`PATCH /api/organizations/acme/teams/t1/ {"name":"platform"}`}, api.writes)
	assert.True(t, out.OrganizationAdmin.ValueBool())
}

func TestTeamDemotionSendsOnlyTheFlag(t *testing.T) {
	api := &adminAPI{teamAdmin: true}
	out := updateTeam(t, api, teamModel("ops", types.BoolValue(true)), teamModel("ops", types.BoolValue(false)))
	assert.Equal(t, []string{`PATCH /api/organizations/acme/teams/t1/ {"organization_admin":false}`}, api.writes)
	assert.False(t, out.OrganizationAdmin.ValueBool())
}

func TestTeamCreateAsAdministrator(t *testing.T) {
	api := &adminAPI{}
	ctx := context.Background()
	r := &TeamResource{provider: adminProvider(api)}
	s := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, s)
	plan := teamModel("ops", types.BoolValue(true))
	plan.ID = types.StringUnknown()
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.Equal(t, []string{`POST /api/organizations/acme/teams/ {"name":"ops","organization_admin":true}`}, api.writes)
	var out TeamResourceModel
	require.Empty(t, resp.State.Get(ctx, &out))
	assert.True(t, out.OrganizationAdmin.ValueBool())
}

func userModel(admin types.Bool) UserResourceModel {
	perm := types.ObjectType{AttrTypes: map[string]attr.Type{"user": types.StringType, "permission": types.StringType,
		"organization": types.StringType, "workspace": types.StringType}}
	return UserResourceModel{ID: types.StringUnknown(), OrganizationName: types.StringValue("acme"),
		Email: types.StringValue("dev@acme.com"), LastLogin: types.StringUnknown(),
		Teams: types.ListUnknown(types.StringType), Permissions: types.ListUnknown(perm), OrganizationAdmin: admin}
}

func createUser(t *testing.T, api *adminAPI, plan UserResourceModel) UserResourceModel {
	t.Helper()
	ctx := context.Background()
	r := &UserResource{provider: adminProvider(api)}
	s := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, s)
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	var out UserResourceModel
	require.Empty(t, resp.State.Get(ctx, &out))
	return out
}

func TestUserCreateAsAdministrator(t *testing.T) {
	api := &adminAPI{}
	out := createUser(t, api, userModel(types.BoolValue(true)))
	assert.Equal(t, []string{
		`POST /api/users/acme/users/ {"members":["dev@acme.com"]}`,
		`PATCH /api/users/acme/users/dev@acme.com/ {"organization_admin":true}`,
	}, api.writes)
	assert.True(t, out.OrganizationAdmin.ValueBool())
}

func TestUserCreateWithoutTheAttributeLeavesAdminAlone(t *testing.T) {
	// The org creator imported as a plain infradots_user must not be demoted by omission.
	api := &adminAPI{userAdmin: true}
	out := createUser(t, api, userModel(types.BoolUnknown()))
	assert.Equal(t, []string{`POST /api/users/acme/users/ {"members":["dev@acme.com"]}`}, api.writes)
	assert.True(t, out.OrganizationAdmin.ValueBool())
}

func TestUserUpdateDemotes(t *testing.T) {
	api := &adminAPI{userAdmin: true}
	ctx := context.Background()
	r := &UserResource{provider: adminProvider(api)}
	s := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, s)
	state := createUser(t, &adminAPI{userAdmin: true}, userModel(types.BoolUnknown()))
	plan := state
	plan.OrganizationAdmin = types.BoolValue(false)
	req := resource.UpdateRequest{State: tfsdk.State{Schema: s.Schema}, Plan: tfsdk.Plan{Schema: s.Schema}}
	require.Empty(t, req.State.Set(ctx, &state))
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Update(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.Equal(t, []string{`PATCH /api/users/acme/users/dev@acme.com/ {"organization_admin":false}`}, api.writes)
	var out UserResourceModel
	require.Empty(t, resp.State.Get(ctx, &out))
	assert.False(t, out.OrganizationAdmin.ValueBool())
}

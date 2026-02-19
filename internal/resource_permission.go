package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource              = &PermissionResource{}
	_ resource.ResourceWithConfigure = &PermissionResource{}
)

func NewPermissionResource() resource.Resource {
	return &PermissionResource{}
}

type PermissionResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	TeamID           types.String `tfsdk:"team_id"`
	UserEmail        types.String `tfsdk:"user_email"`
	Permission       types.String `tfsdk:"permission"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
}

type PermissionAPIResponse struct {
	Permission   string `json:"permission"`
	User         string `json:"user,omitempty"`
	Team         string `json:"team,omitempty"`
	Organization string `json:"organization"`
	Workspace    string `json:"workspace,omitempty"`
}

type OrgPermissionRequest struct {
	Team                string   `json:"team,omitempty"`
	User                string   `json:"user,omitempty"`
	AssignedPermissions []string `json:"assigned_permissions"`
}

type WorkspacePermissionRequest struct {
	Team       string              `json:"team,omitempty"`
	User       string              `json:"user,omitempty"`
	Workspaces map[string][]string `json:"workspaces"`
}

type PermissionResource struct {
	provider *InfradotsProvider
}

func (r *PermissionResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_permission"
}

func (r *PermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Permission mapping for a user or team in an InfraDots organization",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite ID for this permission (organization:permission:user_or_team[:workspace]).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization.",
				Required:    true,
			},
			"team_id": schema.StringAttribute{
				Description: "The ID of the team to assign the permission to. Mutually exclusive with user_email.",
				Optional:    true,
			},
			"user_email": schema.StringAttribute{
				Description: "The email of the user to assign the permission to. Mutually exclusive with team_id.",
				Optional:    true,
			},
			"permission": schema.StringAttribute{
				Description: "The permission codename (e.g., read_workspaces, write_workspaces, read_organizations, write_organizations, read_teams, write_teams).",
				Required:    true,
			},
			"workspace_name": schema.StringAttribute{
				Description: "The workspace name to scope this permission to. If not set, the permission is organization-level.",
				Optional:    true,
			},
		},
	}
}

func (r *PermissionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *PermissionResource) computeID(data *PermissionResourceModel) string {
	parts := []string{
		data.OrganizationName.ValueString(),
		data.Permission.ValueString(),
	}
	if !data.UserEmail.IsNull() && data.UserEmail.ValueString() != "" {
		parts = append(parts, "user:"+data.UserEmail.ValueString())
	} else if !data.TeamID.IsNull() && data.TeamID.ValueString() != "" {
		parts = append(parts, "team:"+data.TeamID.ValueString())
	}
	if !data.WorkspaceName.IsNull() && data.WorkspaceName.ValueString() != "" {
		parts = append(parts, data.WorkspaceName.ValueString())
	}
	return strings.Join(parts, ":")
}

func (r *PermissionResource) readExistingOrgPermissions(data *PermissionResourceModel) ([]string, error) {
	apiUrl := fmt.Sprintf("https://%s/api/permissions/%s/",
		r.provider.host, data.OrganizationName.ValueString())

	u, _ := url.Parse(apiUrl)
	q := u.Query()
	if !data.TeamID.IsNull() && data.TeamID.ValueString() != "" {
		q.Set("team", data.TeamID.ValueString())
	}
	if !data.UserEmail.IsNull() && data.UserEmail.ValueString() != "" {
		q.Set("user", data.UserEmail.ValueString())
	}
	u.RawQuery = q.Encode()

	reqHttp, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		return []string{}, nil
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	var permissions []PermissionAPIResponse
	if err := json.Unmarshal(respBody, &permissions); err != nil {
		return []string{}, nil
	}

	var existing []string
	for _, p := range permissions {
		existing = append(existing, p.Permission)
	}
	return existing, nil
}

func (r *PermissionResource) sendOrgPermissions(data *PermissionResourceModel, perms []string) (int, string, error) {
	apiUrl := fmt.Sprintf("https://%s/api/permissions/%s/",
		r.provider.host, data.OrganizationName.ValueString())

	orgReq := OrgPermissionRequest{
		AssignedPermissions: perms,
	}
	if !data.TeamID.IsNull() && data.TeamID.ValueString() != "" {
		orgReq.Team = data.TeamID.ValueString()
	}
	if !data.UserEmail.IsNull() && data.UserEmail.ValueString() != "" {
		orgReq.User = data.UserEmail.ValueString()
	}

	reqBody, err := json.Marshal(orgReq)
	if err != nil {
		return 0, "", err
	}

	reqHttp, err := http.NewRequest(http.MethodPost, apiUrl, strings.NewReader(string(reqBody)))
	if err != nil {
		return 0, "", err
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)
	reqHttp.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		return 0, "", err
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)
	return httpResp.StatusCode, string(respBody), nil
}

func (r *PermissionResource) sendWorkspacePermissions(data *PermissionResourceModel, perms []string) (int, string, error) {
	apiUrl := fmt.Sprintf("https://%s/api/permissions/%s/workspaces/",
		r.provider.host, data.OrganizationName.ValueString())

	wsReq := WorkspacePermissionRequest{
		Workspaces: map[string][]string{
			data.WorkspaceName.ValueString(): perms,
		},
	}
	if !data.TeamID.IsNull() && data.TeamID.ValueString() != "" {
		wsReq.Team = data.TeamID.ValueString()
	}
	if !data.UserEmail.IsNull() && data.UserEmail.ValueString() != "" {
		wsReq.User = data.UserEmail.ValueString()
	}

	reqBody, err := json.Marshal(wsReq)
	if err != nil {
		return 0, "", err
	}

	reqHttp, err := http.NewRequest(http.MethodPost, apiUrl, strings.NewReader(string(reqBody)))
	if err != nil {
		return 0, "", err
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)
	reqHttp.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		return 0, "", err
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)
	return httpResp.StatusCode, string(respBody), nil
}

func addToSet(existing []string, perm string) []string {
	for _, p := range existing {
		if p == perm {
			return existing
		}
	}
	return append(existing, perm)
}

func removeFromSet(existing []string, perm string) []string {
	var result []string
	for _, p := range existing {
		if p != perm {
			result = append(result, p)
		}
	}
	return result
}

func (r *PermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PermissionResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if (data.TeamID.IsNull() || data.TeamID.ValueString() == "") && (data.UserEmail.IsNull() || data.UserEmail.ValueString() == "") {
		resp.Diagnostics.AddError("Invalid configuration", "Either team_id or user_email must be specified")
		return
	}

	isWorkspaceLevel := !data.WorkspaceName.IsNull() && data.WorkspaceName.ValueString() != ""

	if isWorkspaceLevel {
		statusCode, body, err := r.sendWorkspacePermissions(&data, []string{data.Permission.ValueString()})
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		if statusCode != 201 && statusCode != 200 {
			resp.Diagnostics.AddError("Create permission failed", fmt.Sprintf("Status: %d, Body: %s", statusCode, body))
			return
		}
	} else {
		existing, err := r.readExistingOrgPermissions(&data)
		if err != nil {
			resp.Diagnostics.AddError("Error reading existing permissions", err.Error())
			return
		}
		merged := addToSet(existing, data.Permission.ValueString())
		statusCode, body, err := r.sendOrgPermissions(&data, merged)
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		if statusCode != 200 && statusCode != 201 {
			resp.Diagnostics.AddError("Create permission failed", fmt.Sprintf("Status: %d, Body: %s", statusCode, body))
			return
		}
	}

	data.ID = types.StringValue(r.computeID(&data))
	diags = resp.State.Set(ctx, &data)
	tflog.Info(ctx, "Permission Resource Created", map[string]any{"success": true})
	resp.Diagnostics.Append(diags...)
}

func (r *PermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PermissionResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine which endpoint to use
	var apiUrl string
	if !data.WorkspaceName.IsNull() && data.WorkspaceName.ValueString() != "" {
		apiUrl = fmt.Sprintf("https://%s/api/permissions/%s/workspaces/",
			r.provider.host,
			data.OrganizationName.ValueString())
	} else {
		apiUrl = fmt.Sprintf("https://%s/api/permissions/%s/",
			r.provider.host,
			data.OrganizationName.ValueString())
	}

	// Add query params to filter
	u, _ := url.Parse(apiUrl)
	q := u.Query()
	if !data.UserEmail.IsNull() && data.UserEmail.ValueString() != "" {
		q.Set("user", data.UserEmail.ValueString())
	}
	if !data.TeamID.IsNull() && data.TeamID.ValueString() != "" {
		q.Set("team", data.TeamID.ValueString())
	}
	if !data.WorkspaceName.IsNull() && data.WorkspaceName.ValueString() != "" {
		q.Set("workspace", data.WorkspaceName.ValueString())
	}
	u.RawQuery = q.Encode()

	reqHttp, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if httpResp.StatusCode >= 500 {
		// Server error -- retain existing state so destroy can proceed
		data.ID = types.StringValue(r.computeID(&data))
		diags = resp.State.Set(ctx, &data)
		resp.Diagnostics.Append(diags...)
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var permissions []PermissionAPIResponse
	err = json.Unmarshal(respBody, &permissions)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Find the matching permission
	found := false
	for _, perm := range permissions {
		if perm.Permission == data.Permission.ValueString() {
			found = true
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(r.computeID(&data))
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *PermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state PermissionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan PermissionResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	isWorkspaceLevel := !plan.WorkspaceName.IsNull() && plan.WorkspaceName.ValueString() != ""

	if isWorkspaceLevel {
		statusCode, body, err := r.sendWorkspacePermissions(&plan, []string{plan.Permission.ValueString()})
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		if statusCode != 201 && statusCode != 200 {
			resp.Diagnostics.AddError("Update permission failed", fmt.Sprintf("Status: %d, Body: %s", statusCode, body))
			return
		}
	} else {
		existing, err := r.readExistingOrgPermissions(&plan)
		if err != nil {
			resp.Diagnostics.AddError("Error reading existing permissions", err.Error())
			return
		}
		existing = removeFromSet(existing, state.Permission.ValueString())
		merged := addToSet(existing, plan.Permission.ValueString())
		statusCode, body, err := r.sendOrgPermissions(&plan, merged)
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		if statusCode != 200 && statusCode != 201 {
			resp.Diagnostics.AddError("Update permission failed", fmt.Sprintf("Status: %d, Body: %s", statusCode, body))
			return
		}
	}

	plan.ID = types.StringValue(r.computeID(&plan))
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *PermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PermissionResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	isWorkspaceLevel := !data.WorkspaceName.IsNull() && data.WorkspaceName.ValueString() != ""

	if isWorkspaceLevel {
		// Send empty permissions for this workspace to remove all
		statusCode, body, err := r.sendWorkspacePermissions(&data, []string{})
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		if statusCode != 201 && statusCode != 200 {
			resp.Diagnostics.AddError("Delete permission failed", fmt.Sprintf("Status: %d, Body: %s", statusCode, body))
			return
		}
	} else {
		existing, err := r.readExistingOrgPermissions(&data)
		if err != nil {
			resp.Diagnostics.AddError("Error reading existing permissions", err.Error())
			return
		}
		remaining := removeFromSet(existing, data.Permission.ValueString())
		statusCode, body, err := r.sendOrgPermissions(&data, remaining)
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		if statusCode != 200 && statusCode != 201 {
			resp.Diagnostics.AddError("Delete permission failed", fmt.Sprintf("Status: %d, Body: %s", statusCode, body))
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *PermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"Import not supported",
		"Permission resources cannot be imported. Please define them in your configuration.",
	)
}

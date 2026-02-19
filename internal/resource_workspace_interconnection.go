package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource              = &WorkspaceInterconnectionResource{}
	_ resource.ResourceWithConfigure = &WorkspaceInterconnectionResource{}
)

func NewWorkspaceInterconnectionResource() resource.Resource {
	return &WorkspaceInterconnectionResource{}
}

type WorkspaceInterconnectionResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
	ConnectedTo      types.List   `tfsdk:"connected_to"`
	Condition        types.String `tfsdk:"condition"`
}

type InterconnectionListResponse struct {
	ID                    int                        `json:"id"`
	Condition             string                     `json:"condition"`
	ConnectedWorkspaces   []InterconnectedWorkspace  `json:"connected_workspaces"`
}

type InterconnectedWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type InterconnectionConnectRequest struct {
	Workspaces []string `json:"workspaces"`
	Condition  string   `json:"condition,omitempty"`
}

type InterconnectionConnectResponse struct {
	Connected           []string `json:"connected"`
	WorkspaceNotExisting []string `json:"workspace_not_existing"`
}

type WorkspaceInterconnectionResource struct {
	provider *InfradotsProvider
}

func (r *WorkspaceInterconnectionResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_workspace_interconnection"
}

func (r *WorkspaceInterconnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Workspace interconnection for multi-workspace orchestration in InfraDots",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite ID for this interconnection (organization_name:workspace_name).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization.",
				Required:    true,
			},
			"workspace_name": schema.StringAttribute{
				Description: "The source workspace name (runs in this workspace trigger connected workspaces).",
				Required:    true,
			},
			"connected_to": schema.ListAttribute{
				Description: "List of workspace names that this workspace triggers.",
				ElementType: types.StringType,
				Required:    true,
			},
			"condition": schema.StringAttribute{
				Description: "Condition for triggering connected workspaces: full_apply or always.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("full_apply"),
				Validators: []validator.String{
					stringvalidator.OneOf("full_apply", "always"),
				},
			},
		},
	}
}

func (r *WorkspaceInterconnectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *WorkspaceInterconnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceInterconnectionResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var connectedTo []string
	diags = data.ConnectedTo.ElementsAs(ctx, &connectedTo, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	connectReq := InterconnectionConnectRequest{
		Workspaces: connectedTo,
		Condition:  data.Condition.ValueString(),
	}

	reqBody, err := json.Marshal(connectReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/connect_workspaces/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString())

	reqHttp, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)
	reqHttp.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError(
			"Create interconnection failed",
			fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	data.ID = types.StringValue(data.OrganizationName.ValueString() + ":" + data.WorkspaceName.ValueString())

	// Read back the actual state
	r.readInterconnection(ctx, &data, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	tflog.Info(ctx, "Workspace Interconnection Created", map[string]any{"success": true})
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceInterconnectionResource) readInterconnection(ctx context.Context, data *WorkspaceInterconnectionResourceModel, resp interface{}) {
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/connect_workspaces/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString())

	reqHttp, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		addDiagError(resp, "Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		addDiagError(resp, "HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		addDiagError(resp, "Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		addDiagError(resp, "Error reading response body", err.Error())
		return
	}

	var inter InterconnectionListResponse
	err = json.Unmarshal(respBody, &inter)
	if err != nil {
		addDiagError(resp, "Error parsing response", err.Error())
		return
	}

	if inter.Condition != "" {
		data.Condition = types.StringValue(inter.Condition)
	}

	wsNames := make([]attr.Value, 0, len(inter.ConnectedWorkspaces))
	for _, ws := range inter.ConnectedWorkspaces {
		wsNames = append(wsNames, types.StringValue(ws.Name))
	}
	data.ConnectedTo = types.ListValueMust(types.StringType, wsNames)
	data.ID = types.StringValue(data.OrganizationName.ValueString() + ":" + data.WorkspaceName.ValueString())
}

func addDiagError(resp interface{}, summary, detail string) {
	switch r := resp.(type) {
	case *resource.CreateResponse:
		r.Diagnostics.AddError(summary, detail)
	case *resource.ReadResponse:
		r.Diagnostics.AddError(summary, detail)
	case *resource.UpdateResponse:
		r.Diagnostics.AddError(summary, detail)
	}
}

func (r *WorkspaceInterconnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceInterconnectionResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readInterconnection(ctx, &data, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	// If no connected workspaces, remove from state
	if data.ConnectedTo.IsNull() || len(data.ConnectedTo.Elements()) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceInterconnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state WorkspaceInterconnectionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WorkspaceInterconnectionResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Disconnect all existing
	var currentConnected []string
	diags = state.ConnectedTo.ElementsAs(ctx, &currentConnected, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(currentConnected) > 0 {
		disconnectReq := InterconnectionConnectRequest{
			Workspaces: currentConnected,
		}
		reqBody, _ := json.Marshal(disconnectReq)

		url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/connect_workspaces/",
			r.provider.host,
			plan.OrganizationName.ValueString(),
			plan.WorkspaceName.ValueString())

		reqHttp, err := http.NewRequest(http.MethodDelete, url, strings.NewReader(string(reqBody)))
		if err != nil {
			resp.Diagnostics.AddError("Error creating request", err.Error())
			return
		}
		reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)
		reqHttp.Header.Set("Content-Type", "application/json")

		httpResp, err := r.provider.client.Do(reqHttp)
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		defer httpResp.Body.Close()
	}

	// Connect new set
	var newConnected []string
	diags = plan.ConnectedTo.ElementsAs(ctx, &newConnected, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	connectReq := InterconnectionConnectRequest{
		Workspaces: newConnected,
		Condition:  plan.Condition.ValueString(),
	}
	reqBody, _ := json.Marshal(connectReq)

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/connect_workspaces/",
		r.provider.host,
		plan.OrganizationName.ValueString(),
		plan.WorkspaceName.ValueString())

	reqHttp, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)
	reqHttp.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
		respBody, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"Update interconnection failed",
			fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	// Read back the actual state
	r.readInterconnection(ctx, &plan, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceInterconnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceInterconnectionResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var connectedTo []string
	diags = data.ConnectedTo.ElementsAs(ctx, &connectedTo, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(connectedTo) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	disconnectReq := InterconnectionConnectRequest{
		Workspaces: connectedTo,
	}
	reqBody, _ := json.Marshal(disconnectReq)

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/connect_workspaces/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString())

	reqHttp, err := http.NewRequest(http.MethodDelete, url, strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)
	reqHttp.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 204 {
		resp.Diagnostics.AddError(
			"Delete failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *WorkspaceInterconnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:workspace_name'",
		)
		return
	}

	organizationName := parts[0]
	workspaceName := parts[1]

	var data WorkspaceInterconnectionResourceModel
	data.OrganizationName = types.StringValue(organizationName)
	data.WorkspaceName = types.StringValue(workspaceName)

	readResp := &resource.ReadResponse{
		State: resp.State,
	}
	r.readInterconnection(ctx, &data, readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

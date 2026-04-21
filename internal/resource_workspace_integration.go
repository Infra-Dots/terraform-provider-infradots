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
)

var _ resource.Resource = &WorkspaceIntegrationResource{}

func NewWorkspaceIntegrationResource() resource.Resource {
	return &WorkspaceIntegrationResource{}
}

type WorkspaceIntegrationResource struct {
	provider *InfradotsProvider
}

type WorkspaceIntegrationResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
	IntegrationID    types.String `tfsdk:"integration_id"`
	RunAfterStage    types.String `tfsdk:"run_after_stage"`
	SlackChannels    types.List   `tfsdk:"slack_channels"`
	SlackEnvChannels types.Map    `tfsdk:"slack_env_channels"`
}

type WorkspaceIntegrationRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type WorkspaceIntegrationAPIResponse struct {
	ID               string                  `json:"id"`
	Integration      WorkspaceIntegrationRef `json:"integration"`
	RunAfterStage    string                  `json:"run_after_stage"`
	SlackChannels    []string                `json:"slack_channels"`
	SlackEnvChannels map[string]string       `json:"slack_env_channels"`
}

type WorkspaceIntegrationCreateRequest struct {
	IntegrationID    string            `json:"integration_id"`
	RunAfterStage    string            `json:"run_after_stage,omitempty"`
	SlackChannels    []string          `json:"slack_channels,omitempty"`
	SlackEnvChannels map[string]string `json:"slack_env_channels,omitempty"`
}

func (r *WorkspaceIntegrationResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_workspace_integration"
}

func (r *WorkspaceIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Attaches an integration to a workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the workspace integration attachment.",
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
				Description: "The name of the workspace.",
				Required:    true,
			},
			"integration_id": schema.StringAttribute{
				Description: "The ID of the integration to attach.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"run_after_stage": schema.StringAttribute{
				Description: "The stage after which the integration runs. One of: init, debug, details, plan, apply, all.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("apply"),
				Validators: []validator.String{
					stringvalidator.OneOf("init", "debug", "details", "plan", "apply", "all"),
				},
			},
			"slack_channels": schema.ListAttribute{
				Description: "List of Slack channel names.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"slack_env_channels": schema.MapAttribute{
				Description: "Map of environment names to Slack channel names.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *WorkspaceIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func mapWorkspaceIntegrationToModel(_ context.Context, data *WorkspaceIntegrationResourceModel, wi WorkspaceIntegrationAPIResponse) {
	data.ID = types.StringValue(wi.ID)
	data.IntegrationID = types.StringValue(wi.Integration.ID)
	data.RunAfterStage = types.StringValue(wi.RunAfterStage)

	channels := wi.SlackChannels
	if channels == nil {
		channels = []string{}
	}
	chanVals := make([]attr.Value, len(channels))
	for i, c := range channels {
		chanVals[i] = types.StringValue(c)
	}
	data.SlackChannels = types.ListValueMust(types.StringType, chanVals)

	if wi.SlackEnvChannels != nil {
		mapVal := map[string]attr.Value{}
		for k, v := range wi.SlackEnvChannels {
			mapVal[k] = types.StringValue(v)
		}
		data.SlackEnvChannels = types.MapValueMust(types.StringType, mapVal)
	} else {
		data.SlackEnvChannels = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
}

func (r *WorkspaceIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceIntegrationResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := WorkspaceIntegrationCreateRequest{
		IntegrationID: data.IntegrationID.ValueString(),
		RunAfterStage: data.RunAfterStage.ValueString(),
	}

	if !data.SlackChannels.IsNull() && !data.SlackChannels.IsUnknown() {
		var channels []string
		diags = data.SlackChannels.ElementsAs(ctx, &channels, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.SlackChannels = channels
	}

	if !data.SlackEnvChannels.IsNull() && !data.SlackEnvChannels.IsUnknown() {
		var envChannels map[string]string
		diags = data.SlackEnvChannels.ElementsAs(ctx, &envChannels, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.SlackEnvChannels = envChannels
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/integrations/%s/attach/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString(),
		data.IntegrationID.ValueString())

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
			"Create failed",
			fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	// The attach endpoint may return an empty body on success; fall back to a GET.
	var wi WorkspaceIntegrationAPIResponse
	if len(respBody) > 0 {
		err = json.Unmarshal(respBody, &wi)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}
		mapWorkspaceIntegrationToModel(ctx, &data, wi)
	} else {
		listURL := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/integrations/",
			r.provider.host,
			data.OrganizationName.ValueString(),
			data.WorkspaceName.ValueString())

		listReq, err := http.NewRequest(http.MethodGet, listURL, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error creating request", err.Error())
			return
		}
		listReq.Header.Set("Authorization", "Bearer "+r.provider.token)

		listResp, err := r.provider.client.Do(listReq)
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		defer listResp.Body.Close()

		listBody, err := io.ReadAll(listResp.Body)
		if err != nil {
			resp.Diagnostics.AddError("Error reading response body", err.Error())
			return
		}

		if listResp.StatusCode != 200 {
			resp.Diagnostics.AddError("Failed to read integration after attach",
				fmt.Sprintf("Status: %d, Body: %s", listResp.StatusCode, string(listBody)))
			return
		}

		var integrations []WorkspaceIntegrationAPIResponse
		if err = json.Unmarshal(listBody, &integrations); err != nil {
			resp.Diagnostics.AddError("Error parsing integrations list", err.Error())
			return
		}

		found := false
		for _, item := range integrations {
			if item.Integration.ID == data.IntegrationID.ValueString() {
				mapWorkspaceIntegrationToModel(ctx, &data, item)
				found = true
				break
			}
		}
		if !found {
			resp.Diagnostics.AddError("Integration not found after attach",
				fmt.Sprintf("Integration %s not found in workspace %s", data.IntegrationID.ValueString(), data.WorkspaceName.ValueString()))
			return
		}
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceIntegrationResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/integrations/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString())

	reqHttp, err := http.NewRequest(http.MethodGet, url, nil)
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

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var integrations []WorkspaceIntegrationAPIResponse
	err = json.Unmarshal(respBody, &integrations)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	found := false
	for _, wi := range integrations {
		if wi.Integration.ID == data.IntegrationID.ValueString() {
			mapWorkspaceIntegrationToModel(ctx, &data, wi)
			found = true
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update is not supported; all fields are immutable or ForceNew.
func (r *WorkspaceIntegrationResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "infradots_workspace_integration does not support updates; all fields are ForceNew.")
}

func (r *WorkspaceIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceIntegrationResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/integrations/%s/detach/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString(),
		data.IntegrationID.ValueString())

	reqHttp, err := http.NewRequest(http.MethodDelete, url, nil)
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

func (r *WorkspaceIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Format: org:workspace:integration_id
	parts := strings.Split(req.ID, ":")
	if len(parts) != 3 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:workspace_name:integration_id'",
		)
		return
	}

	organizationName := parts[0]
	workspaceName := parts[1]
	integrationID := parts[2]

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/integrations/",
		r.provider.host,
		organizationName,
		workspaceName)

	reqHttp, err := http.NewRequest(http.MethodGet, url, nil)
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

	if httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"Failed to fetch integrations",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var integrations []WorkspaceIntegrationAPIResponse
	err = json.Unmarshal(respBody, &integrations)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var data WorkspaceIntegrationResourceModel
	data.OrganizationName = types.StringValue(organizationName)
	data.WorkspaceName = types.StringValue(workspaceName)

	found := false
	for _, wi := range integrations {
		if wi.Integration.ID == integrationID {
			mapWorkspaceIntegrationToModel(ctx, &data, wi)
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Integration not found",
			fmt.Sprintf("No integration with ID '%s' found in workspace '%s' of organization '%s'",
				integrationID, workspaceName, organizationName),
		)
		return
	}

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

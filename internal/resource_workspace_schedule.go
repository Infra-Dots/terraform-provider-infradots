package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &WorkspaceScheduleResource{}

func NewWorkspaceScheduleResource() resource.Resource {
	return &WorkspaceScheduleResource{}
}

type WorkspaceScheduleResource struct {
	provider *InfradotsProvider
}

type WorkspaceScheduleResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
	Type             types.String `tfsdk:"type"`
	Crontab          types.String `tfsdk:"crontab"`
	Schedule         types.String `tfsdk:"schedule"`
}

type WorkspaceScheduleAPIResponse struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Crontab  string `json:"crontab"`
	Schedule string `json:"schedule"`
}

type WorkspaceScheduleCreateRequest struct {
	Type    string `json:"type"`
	Crontab string `json:"crontab"`
}

type WorkspaceScheduleUpdateRequest struct {
	Type    string `json:"type,omitempty"`
	Crontab string `json:"crontab,omitempty"`
}

func (r *WorkspaceScheduleResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_workspace_schedule"
}

func (r *WorkspaceScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a cron schedule for a workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the workspace schedule.",
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
			"type": schema.StringAttribute{
				Description: "The schedule type. One of: plan, apply, destroy, refresh.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("plan", "apply", "destroy", "refresh"),
				},
			},
			"crontab": schema.StringAttribute{
				Description: "Cron expression in the format 'minute hour day_of_month month_of_year day_of_week' (e.g., '0 12 * * *').",
				Required:    true,
			},
			"schedule": schema.StringAttribute{
				Description: "Human-readable representation of the schedule returned by the API.",
				Computed:    true,
			},
		},
	}
}

func (r *WorkspaceScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *WorkspaceScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceScheduleResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := WorkspaceScheduleCreateRequest{
		Type:    data.Type.ValueString(),
		Crontab: data.Crontab.ValueString(),
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/schedules/",
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

	if httpResp.StatusCode != 201 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Create failed",
			fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	var schedule WorkspaceScheduleAPIResponse
	err = json.Unmarshal(respBody, &schedule)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(schedule.ID)
	data.Type = types.StringValue(schedule.Type)
	// crontab may not be returned by the API; preserve plan value if empty
	if schedule.Crontab != "" {
		data.Crontab = types.StringValue(schedule.Crontab)
	}
	data.Schedule = types.StringValue(schedule.Schedule)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceScheduleResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/schedules/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString(),
		data.ID.ValueString())

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

	var schedule WorkspaceScheduleAPIResponse
	err = json.Unmarshal(respBody, &schedule)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(schedule.ID)
	data.Type = types.StringValue(schedule.Type)
	// crontab may not be returned by GET; preserve existing state value
	if schedule.Crontab != "" {
		data.Crontab = types.StringValue(schedule.Crontab)
	}
	data.Schedule = types.StringValue(schedule.Schedule)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state WorkspaceScheduleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WorkspaceScheduleResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := WorkspaceScheduleUpdateRequest{}

	if !plan.Type.Equal(state.Type) {
		updateReq.Type = plan.Type.ValueString()
	}
	if !plan.Crontab.Equal(state.Crontab) {
		updateReq.Crontab = plan.Crontab.ValueString()
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/schedules/%s/",
		r.provider.host,
		plan.OrganizationName.ValueString(),
		plan.WorkspaceName.ValueString(),
		state.ID.ValueString())

	reqHttp, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(string(reqBody)))
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

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Update failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	var schedule WorkspaceScheduleAPIResponse
	err = json.Unmarshal(respBody, &schedule)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(schedule.ID)
	plan.Type = types.StringValue(schedule.Type)
	plan.Crontab = types.StringValue(schedule.Crontab)
	plan.Schedule = types.StringValue(schedule.Schedule)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceScheduleResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/schedules/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString(),
		data.ID.ValueString())

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

func (r *WorkspaceScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Format: org:workspace:id
	parts := strings.Split(req.ID, ":")
	if len(parts) != 3 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:workspace_name:id'",
		)
		return
	}

	organizationName := parts[0]
	workspaceName := parts[1]
	id := parts[2]

	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/schedules/%s/",
		r.provider.host,
		organizationName,
		workspaceName,
		id)

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
			"Failed to fetch workspace schedule",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var schedule WorkspaceScheduleAPIResponse
	err = json.Unmarshal(respBody, &schedule)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var data WorkspaceScheduleResourceModel
	data.OrganizationName = types.StringValue(organizationName)
	data.WorkspaceName = types.StringValue(workspaceName)
	data.ID = types.StringValue(schedule.ID)
	data.Type = types.StringValue(schedule.Type)
	data.Crontab = types.StringValue(schedule.Crontab)
	data.Schedule = types.StringValue(schedule.Schedule)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

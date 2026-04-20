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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource              = &WorkerPoolResource{}
	_ resource.ResourceWithConfigure = &WorkerPoolResource{}
)

func NewWorkerPoolResource() resource.Resource {
	return &WorkerPoolResource{}
}

type WorkerPoolResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	OrganizationName   types.String `tfsdk:"organization_name"`
	Name               types.String `tfsdk:"name"`
	RegistrationToken  types.String `tfsdk:"registration_token"`
	RestrictToAssigned types.Bool   `tfsdk:"restrict_to_assigned"`
}

type WorkerPoolAPIResponse struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	RegistrationToken  string `json:"registration_token,omitempty"`
	WorkersCount       int    `json:"workers_count"`
	RestrictToAssigned bool   `json:"restrict_to_assigned"`
}

type WorkerPoolCreateRequest struct {
	Name               string `json:"name"`
	RestrictToAssigned bool   `json:"restrict_to_assigned"`
}

type WorkerPoolUpdateRequest struct {
	Name               string `json:"name,omitempty"`
	RestrictToAssigned *bool  `json:"restrict_to_assigned,omitempty"`
}

type WorkerPoolResource struct {
	provider *InfradotsProvider
}

func (r *WorkerPoolResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_worker_pool"
}

func (r *WorkerPoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Worker pool in an InfraDots organization for remote execution",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The worker pool unique ID (UUID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this worker pool belongs to.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the worker pool.",
				Required:    true,
			},
			"registration_token": schema.StringAttribute{
				Description: "The registration token for workers to join this pool. Only available after creation.",
				Computed:    true,
				Sensitive:   true,
			},
			"restrict_to_assigned": schema.BoolAttribute{
				Description: "Whether to restrict this pool to only assigned workspaces.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

func (r *WorkerPoolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *WorkerPoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkerPoolResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := WorkerPoolCreateRequest{
		Name:               data.Name.ValueString(),
		RestrictToAssigned: data.RestrictToAssigned.ValueBool(),
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/workers/%s/pools/",
		r.provider.host,
		data.OrganizationName.ValueString())

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

	if httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError(
			"Non-201 response",
			fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	var pool WorkerPoolAPIResponse
	err = json.Unmarshal(respBody, &pool)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(pool.ID)
	data.Name = types.StringValue(pool.Name)
	data.RestrictToAssigned = types.BoolValue(pool.RestrictToAssigned)
	if pool.RegistrationToken != "" {
		data.RegistrationToken = types.StringValue(pool.RegistrationToken)
	} else {
		data.RegistrationToken = types.StringValue("")
	}

	diags = resp.State.Set(ctx, &data)
	tflog.Info(ctx, "Worker Pool Resource Created", map[string]any{"success": true})
	resp.Diagnostics.Append(diags...)
}

func (r *WorkerPoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkerPoolResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/workers/%s/pools/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
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

	var pool WorkerPoolAPIResponse
	err = json.Unmarshal(respBody, &pool)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(pool.ID)
	data.Name = types.StringValue(pool.Name)
	data.RestrictToAssigned = types.BoolValue(pool.RestrictToAssigned)
	// registration_token is not returned on read, keep existing value

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkerPoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state WorkerPoolResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WorkerPoolResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := WorkerPoolUpdateRequest{}
	if !plan.Name.Equal(state.Name) {
		updateReq.Name = plan.Name.ValueString()
	}
	if !plan.RestrictToAssigned.Equal(state.RestrictToAssigned) {
		v := plan.RestrictToAssigned.ValueBool()
		updateReq.RestrictToAssigned = &v
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/workers/%s/pools/%s/",
		r.provider.host,
		plan.OrganizationName.ValueString(),
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

	var pool WorkerPoolAPIResponse
	err = json.Unmarshal(respBody, &pool)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(pool.ID)
	plan.Name = types.StringValue(pool.Name)
	plan.RestrictToAssigned = types.BoolValue(pool.RestrictToAssigned)
	// Preserve registration_token from state
	plan.RegistrationToken = state.RegistrationToken

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkerPoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkerPoolResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/workers/%s/pools/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
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

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Delete failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *WorkerPoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:pool_name'",
		)
		return
	}

	organizationName := parts[0]
	poolName := parts[1]

	url := fmt.Sprintf("https://%s/api/workers/%s/pools/",
		r.provider.host,
		organizationName)

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
			"Failed to fetch worker pools",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var pools []WorkerPoolAPIResponse
	err = json.Unmarshal(respBody, &pools)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var found *WorkerPoolAPIResponse
	for i := range pools {
		if pools[i].Name == poolName {
			found = &pools[i]
			break
		}
	}

	if found == nil {
		resp.Diagnostics.AddError(
			"Worker pool not found",
			fmt.Sprintf("No worker pool with name '%s' found in organization '%s'", poolName, organizationName),
		)
		return
	}

	var data WorkerPoolResourceModel
	data.ID = types.StringValue(found.ID)
	data.OrganizationName = types.StringValue(organizationName)
	data.Name = types.StringValue(found.Name)
	data.RestrictToAssigned = types.BoolValue(found.RestrictToAssigned)
	data.RegistrationToken = types.StringValue("")

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

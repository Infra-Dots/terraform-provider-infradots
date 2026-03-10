package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ServiceAccountResource{}
	_ resource.ResourceWithImportState = &ServiceAccountResource{}
)

func NewServiceAccountResource() resource.Resource {
	return &ServiceAccountResource{}
}

type ServiceAccountResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Scopes      types.List   `tfsdk:"scopes"`
	IsActive    types.Bool   `tfsdk:"is_active"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

type ServiceAccountAPIResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Scopes      []string  `json:"scopes"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type ServiceAccountCreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	IsActive    bool     `json:"is_active"`
}

type ServiceAccountUpdateRequest struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

type ServiceAccountResource struct {
	provider *InfradotsProvider
}

func (r *ServiceAccountResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_service_account"
}

func (r *ServiceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Service Account in InfraDots Platform (admin-only).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The service account unique ID (UUID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The unique name of the service account.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the service account.",
				Optional:    true,
				Computed:    true,
			},
			"scopes": schema.ListAttribute{
				Description: "List of scopes assigned to the service account.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the service account is active.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the service account was created.",
				Computed:    true,
			},
		},
	}
}

func (r *ServiceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func serviceAccountToModel(ctx context.Context, data *ServiceAccountResourceModel, sa ServiceAccountAPIResponse) {
	data.ID = types.StringValue(sa.ID)
	data.Name = types.StringValue(sa.Name)
	data.Description = types.StringValue(sa.Description)
	data.IsActive = types.BoolValue(sa.IsActive)
	data.CreatedAt = types.StringValue(sa.CreatedAt.Format(time.RFC3339))
	scopes, _ := types.ListValueFrom(ctx, types.StringType, sa.Scopes)
	data.Scopes = scopes
}

func (r *ServiceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceAccountResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := ServiceAccountCreateRequest{
		Name:     data.Name.ValueString(),
		IsActive: data.IsActive.ValueBool(),
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		createReq.Description = data.Description.ValueString()
	}
	if !data.Scopes.IsNull() && !data.Scopes.IsUnknown() {
		var scopes []string
		resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &scopes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Scopes = scopes
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/", r.provider.host)
	httpReq, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(httpReq)
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

	var sa ServiceAccountAPIResponse
	if err := json.Unmarshal(respBody, &sa); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	serviceAccountToModel(ctx, &data, sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceAccountResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/%s/", r.provider.host, data.ID.ValueString())
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(httpReq)
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

	var sa ServiceAccountAPIResponse
	if err := json.Unmarshal(respBody, &sa); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	serviceAccountToModel(ctx, &data, sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state ServiceAccountResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan ServiceAccountResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := ServiceAccountUpdateRequest{}

	if !plan.Name.Equal(state.Name) {
		updateReq.Name = plan.Name.ValueString()
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = plan.Description.ValueString()
	}
	if !plan.IsActive.Equal(state.IsActive) {
		v := plan.IsActive.ValueBool()
		updateReq.IsActive = &v
	}
	if !plan.Scopes.Equal(state.Scopes) {
		var scopes []string
		resp.Diagnostics.Append(plan.Scopes.ElementsAs(ctx, &scopes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Scopes = scopes
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/%s/", r.provider.host, state.ID.ValueString())
	httpReq, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(httpReq)
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

	var sa ServiceAccountAPIResponse
	if err := json.Unmarshal(respBody, &sa); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	serviceAccountToModel(ctx, &plan, sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServiceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceAccountResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/%s/", r.provider.host, data.ID.ValueString())
	httpReq, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(httpReq)
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

func (r *ServiceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/%s/", r.provider.host, id)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"Import failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var sa ServiceAccountAPIResponse
	if err := json.Unmarshal(respBody, &sa); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var data ServiceAccountResourceModel
	serviceAccountToModel(ctx, &data, sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &IntegrationResource{}
	_ resource.ResourceWithImportState = &IntegrationResource{}
)

func NewIntegrationResource() resource.Resource {
	return &IntegrationResource{}
}

type IntegrationResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	APIURL           types.String `tfsdk:"api_url"`
	APIKey           types.String `tfsdk:"api_key"`
	Description      types.String `tfsdk:"description"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

type IntegrationAPIResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	APIURL      string    `json:"api_url"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type IntegrationCreateRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	APIURL      string `json:"api_url,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	Description string `json:"description,omitempty"`
}

type IntegrationUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	APIURL      string `json:"api_url,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	Description string `json:"description,omitempty"`
}

type IntegrationResource struct {
	provider *InfradotsProvider
}

func (r *IntegrationResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_integration"
}

func (r *IntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Integration for an organization in InfraDots Platform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The integration unique ID (UUID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this integration belongs to.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the integration.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The type of integration. Valid values are: WEBHOOK, CUSTOM, SLACK.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("WEBHOOK", "CUSTOM", "SLACK"),
				},
			},
			"api_url": schema.StringAttribute{
				Description: "The API URL for the integration.",
				Optional:    true,
				Computed:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "The API key for the integration. Write-only; not returned by the API on read.",
				Optional:    true,
				Sensitive:   true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the integration.",
				Optional:    true,
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the integration was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the integration was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *IntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *IntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IntegrationResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := IntegrationCreateRequest{
		Name: data.Name.ValueString(),
		Type: data.Type.ValueString(),
	}
	if !data.APIURL.IsNull() && !data.APIURL.IsUnknown() {
		createReq.APIURL = data.APIURL.ValueString()
	}
	if !data.APIKey.IsNull() && !data.APIKey.IsUnknown() {
		createReq.APIKey = data.APIKey.ValueString()
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		createReq.Description = data.Description.ValueString()
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/integrations/",
		r.provider.host,
		data.OrganizationName.ValueString())
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

	var apiResp IntegrationAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	integrationAPIToModel(&data, apiResp)
	// api_key is preserved from plan (write-only, not returned by API)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IntegrationResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/integrations/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.ID.ValueString())
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

	var apiResp IntegrationAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Preserve api_key from existing state (write-only secret, not returned by API).
	existingAPIKey := data.APIKey
	integrationAPIToModel(&data, apiResp)
	data.APIKey = existingAPIKey

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state IntegrationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan IntegrationResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := IntegrationUpdateRequest{}
	if !plan.Name.Equal(state.Name) {
		updateReq.Name = plan.Name.ValueString()
	}
	if !plan.APIURL.Equal(state.APIURL) {
		updateReq.APIURL = plan.APIURL.ValueString()
	}
	if !plan.APIKey.Equal(state.APIKey) {
		updateReq.APIKey = plan.APIKey.ValueString()
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = plan.Description.ValueString()
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/integrations/%s/",
		r.provider.host,
		state.OrganizationName.ValueString(),
		state.ID.ValueString())
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

	var apiResp IntegrationAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	integrationAPIToModel(&plan, apiResp)
	// api_key is write-only; keep from plan.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IntegrationResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/integrations/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.ID.ValueString())
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

func (r *IntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: "org:integration_id"
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:integration_id'",
		)
		return
	}
	org := parts[0]
	integrationID := parts[1]

	url := fmt.Sprintf("https://%s/api/organizations/%s/integrations/%s/",
		r.provider.host, org, integrationID)
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

	var apiResp IntegrationAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var data IntegrationResourceModel
	data.OrganizationName = types.StringValue(org)
	integrationAPIToModel(&data, apiResp)
	// api_key cannot be recovered on import.
	data.APIKey = types.StringNull()

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func integrationAPIToModel(data *IntegrationResourceModel, apiResp IntegrationAPIResponse) {
	data.ID = types.StringValue(apiResp.ID)
	data.Name = types.StringValue(apiResp.Name)
	data.Type = types.StringValue(apiResp.Type)
	data.APIURL = types.StringValue(apiResp.APIURL)
	data.Description = types.StringValue(apiResp.Description)
	data.CreatedAt = types.StringValue(apiResp.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(apiResp.UpdatedAt.Format(time.RFC3339))
}

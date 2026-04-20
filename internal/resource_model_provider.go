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

var _ resource.Resource = &ModelProviderResource{}

func NewModelProviderResource() resource.Resource {
	return &ModelProviderResource{}
}

type ModelProviderResource struct {
	provider *InfradotsProvider
}

type ModelProviderResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	Provider         types.String `tfsdk:"provider_type"`
	APIKey           types.String `tfsdk:"api_key"`
	Description      types.String `tfsdk:"description"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

type ModelProviderAPIResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ModelProviderCreateRequest struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	APIKey      string `json:"api_key"`
	Description string `json:"description,omitempty"`
}

type ModelProviderUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	Description string `json:"description,omitempty"`
}

func (r *ModelProviderResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_model_provider"
}

func (r *ModelProviderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AI model provider.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the model provider.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this model provider belongs to.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the model provider.",
				Required:    true,
			},
			"provider_type": schema.StringAttribute{
				Description: "The provider type. One of: openai, anthropic, google, azure_openai, cohere, llama.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("openai", "anthropic", "google", "azure_openai", "cohere", "llama"),
				},
			},
			"api_key": schema.StringAttribute{
				Description: "The API key for the model provider. Write-only; never read back from the API.",
				Required:    true,
				Sensitive:   true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the model provider.",
				Optional:    true,
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the model provider was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the model provider was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *ModelProviderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *ModelProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelProviderResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := ModelProviderCreateRequest{
		Name:        data.Name.ValueString(),
		Provider:    data.Provider.ValueString(),
		APIKey:      data.APIKey.ValueString(),
		Description: data.Description.ValueString(),
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/model-providers/",
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

	if httpResp.StatusCode != 201 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Create failed",
			fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	var mp ModelProviderAPIResponse
	err = json.Unmarshal(respBody, &mp)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(mp.ID)
	data.Name = types.StringValue(mp.Name)
	data.Provider = types.StringValue(mp.Provider)
	data.Description = types.StringValue(mp.Description)
	data.CreatedAt = types.StringValue(mp.CreatedAt)
	data.UpdatedAt = types.StringValue(mp.UpdatedAt)
	// api_key is write-only; keep value from plan (already in data)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *ModelProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelProviderResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/model-providers/%s/",
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

	var mp ModelProviderAPIResponse
	err = json.Unmarshal(respBody, &mp)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(mp.ID)
	data.Name = types.StringValue(mp.Name)
	data.Provider = types.StringValue(mp.Provider)
	data.Description = types.StringValue(mp.Description)
	data.CreatedAt = types.StringValue(mp.CreatedAt)
	data.UpdatedAt = types.StringValue(mp.UpdatedAt)
	// api_key is write-only; do NOT overwrite from API response

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *ModelProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state ModelProviderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan ModelProviderResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := ModelProviderUpdateRequest{}

	if !plan.Name.Equal(state.Name) {
		updateReq.Name = plan.Name.ValueString()
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

	url := fmt.Sprintf("https://%s/api/organizations/%s/model-providers/%s/",
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

	var mp ModelProviderAPIResponse
	err = json.Unmarshal(respBody, &mp)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(mp.ID)
	plan.Name = types.StringValue(mp.Name)
	plan.Provider = types.StringValue(mp.Provider)
	plan.Description = types.StringValue(mp.Description)
	plan.CreatedAt = types.StringValue(mp.CreatedAt)
	plan.UpdatedAt = types.StringValue(mp.UpdatedAt)
	// api_key: keep plan value (write-only)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ModelProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelProviderResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/model-providers/%s/",
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

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 204 {
		resp.Diagnostics.AddError(
			"Delete failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *ModelProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Format: org:id
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:id'",
		)
		return
	}

	organizationName := parts[0]
	id := parts[1]

	url := fmt.Sprintf("https://%s/api/organizations/%s/model-providers/%s/",
		r.provider.host,
		organizationName,
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
			"Failed to fetch model provider",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var mp ModelProviderAPIResponse
	err = json.Unmarshal(respBody, &mp)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var data ModelProviderResourceModel
	data.OrganizationName = types.StringValue(organizationName)
	data.ID = types.StringValue(mp.ID)
	data.Name = types.StringValue(mp.Name)
	data.Provider = types.StringValue(mp.Provider)
	data.Description = types.StringValue(mp.Description)
	data.CreatedAt = types.StringValue(mp.CreatedAt)
	data.UpdatedAt = types.StringValue(mp.UpdatedAt)
	// api_key cannot be imported (write-only); leave as unknown/empty
	data.APIKey = types.StringValue("")

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

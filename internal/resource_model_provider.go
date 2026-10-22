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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
	ID                types.String `tfsdk:"id"`
	OrganizationName  types.String `tfsdk:"organization_name"`
	Name              types.String `tfsdk:"name"`
	Provider          types.String `tfsdk:"provider_type"`
	APIKey            types.String `tfsdk:"api_key"`
	Settings          types.Map    `tfsdk:"settings"`
	Config            types.Map    `tfsdk:"config"`
	ModelMap          types.Map    `tfsdk:"model_map"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	VerifiedAt        types.String `tfsdk:"verified_at"`
	VerificationError types.String `tfsdk:"verification_error"`
	Description       types.String `tfsdk:"description"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

type ModelProviderAPIResponse struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Provider          string            `json:"provider"`
	Config            map[string]string `json:"config"`
	ModelMap          map[string]string `json:"model_map"`
	Enabled           bool              `json:"enabled"`
	VerifiedAt        string            `json:"verified_at"`
	VerificationError string            `json:"verification_error"`
	Description       string            `json:"description"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
}

type ModelProviderCreateRequest struct {
	Name        string            `json:"name"`
	Provider    string            `json:"provider"`
	APIKey      string            `json:"api_key,omitempty"`
	Settings    map[string]string `json:"settings,omitempty"`
	ModelMap    map[string]string `json:"model_map,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Description string            `json:"description,omitempty"`
}

type ModelProviderUpdateRequest struct {
	Name        string            `json:"name,omitempty"`
	APIKey      string            `json:"api_key,omitempty"`
	Settings    map[string]string `json:"settings,omitempty"`
	ModelMap    map[string]string `json:"model_map,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Description string            `json:"description,omitempty"`
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
				Description: "Where the organization's model calls go. One of: anthropic, bedrock, vertex, foundry. " +
					"Each runs the same Claude models InfraDots runs, through your own account.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("anthropic", "bedrock", "vertex", "foundry"),
				},
			},
			"api_key": schema.StringAttribute{
				Description: "Shorthand for settings.api_key, for the providers that take one (anthropic, foundry). " +
					"Write-only; never read back from the API.",
				Optional:  true,
				Sensitive: true,
			},
			"settings": schema.MapAttribute{
				Description: "The provider's client settings, by their own names: api_key and base_url for anthropic; " +
					"aws_region, aws_access_key, aws_secret_key and aws_session_token for bedrock; project_id, region " +
					"and access_token for vertex; resource, api_key and base_url for foundry. " +
					"Write-only as a whole, because InfraDots cannot return the secret half; the non-secret half is " +
					"read back under `config`. A provider must carry the credentials that identify its account, or a " +
					"run on it would reach InfraDots' own.",
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
			},
			"config": schema.MapAttribute{
				Description: "The non-secret half of `settings`, as InfraDots holds it (a region, a project, an endpoint).",
				Computed:    true,
				ElementType: types.StringType,
			},
			"model_map": schema.MapAttribute{
				Description: "Each model InfraDots asks for, mapped to this provider's own id, e.g. " +
					"{ \"claude-opus-5\" = \"eu.anthropic.claude-opus-5-v1:0\" }. Required for bedrock, vertex and " +
					"foundry: a run asking for an unmapped model stops before it starts.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether this provider may serve runs. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"verified_at": schema.StringAttribute{
				Description: "When this provider last answered a verification call, if it ever has.",
				Computed:    true,
			},
			"verification_error": schema.StringAttribute{
				Description: "Why the last verification failed, if it did.",
				Computed:    true,
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

	enabled := data.Enabled.ValueBool()
	createReq := ModelProviderCreateRequest{
		Name:        data.Name.ValueString(),
		Provider:    data.Provider.ValueString(),
		APIKey:      data.APIKey.ValueString(),
		Settings:    stringMap(ctx, data.Settings, &resp.Diagnostics),
		ModelMap:    stringMap(ctx, data.ModelMap, &resp.Diagnostics),
		Enabled:     &enabled,
		Description: data.Description.ValueString(),
	}
	if resp.Diagnostics.HasError() {
		return
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

	applyModelProviderResponse(&data, mp)

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

	applyModelProviderResponse(&data, mp)

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
	// Sent only when they changed: writing settings replaces them wholesale and drops the
	// provider's verification, so an unrelated edit must not touch them.
	if !plan.Settings.Equal(state.Settings) {
		updateReq.Settings = stringMap(ctx, plan.Settings, &resp.Diagnostics)
	}
	if !plan.ModelMap.Equal(state.ModelMap) {
		updateReq.ModelMap = stringMap(ctx, plan.ModelMap, &resp.Diagnostics)
	}
	if !plan.Enabled.Equal(state.Enabled) {
		enabled := plan.Enabled.ValueBool()
		updateReq.Enabled = &enabled
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = plan.Description.ValueString()
	}
	if resp.Diagnostics.HasError() {
		return
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

	applyModelProviderResponse(&plan, mp)

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

// stringMap reads a types.Map of strings into a Go map. Null or unknown yields nil, which the
// request structs omit -- not an empty map, which would mean "replace what is stored with nothing".
func stringMap(ctx context.Context, value types.Map, diags *diag.Diagnostics) map[string]string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var out map[string]string
	diags.Append(value.ElementsAs(ctx, &out, false)...)
	return out
}

// mapValue turns an API map into a types.Map, never null: an absent map is an empty one, so a plan
// does not show a change between null and {}.
func mapValue(values map[string]string) types.Map {
	elements := map[string]attr.Value{}
	for key, value := range values {
		elements[key] = types.StringValue(value)
	}
	return types.MapValueMust(types.StringType, elements)
}

// applyModelProviderResponse copies what the API returned onto the model.
//
// `settings` and `api_key` are deliberately not touched. The API cannot return the secret half of
// the settings and does not try to, so overwriting them here would replace the configured value
// with nothing and report "inconsistent values for sensitive attribute" on the next apply -- the
// same trap `variable.value` fell into. The configured value stays; only `config` reads back.
func applyModelProviderResponse(data *ModelProviderResourceModel, mp ModelProviderAPIResponse) {
	data.ID = types.StringValue(mp.ID)
	data.Name = types.StringValue(mp.Name)
	data.Provider = types.StringValue(mp.Provider)
	data.Config = mapValue(mp.Config)
	data.ModelMap = mapValue(mp.ModelMap)
	data.Enabled = types.BoolValue(mp.Enabled)
	data.VerifiedAt = types.StringValue(mp.VerifiedAt)
	data.VerificationError = types.StringValue(mp.VerificationError)
	data.Description = types.StringValue(mp.Description)
	data.CreatedAt = types.StringValue(mp.CreatedAt)
	data.UpdatedAt = types.StringValue(mp.UpdatedAt)
}

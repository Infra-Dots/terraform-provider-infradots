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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure we fully satisfy the resource.Resource interface.
var _ resource.Resource = &VariableResource{}

func NewVariableResource() resource.Resource {
	return &VariableResource{}
}

// VariableResourceModel maps the variable resource schema data.
type VariableResourceModel struct {
	ID               types.String `tfsdk:"id"`                // UUID
	OrganizationName types.String `tfsdk:"organization_name"` // Name of the organization
	Key              types.String `tfsdk:"key"`               // Variable name/key
	Value            types.String `tfsdk:"value"`             // Variable value
	Description      types.String `tfsdk:"description"`       // Optional description
	Category         types.String `tfsdk:"category"`          // E.g., "terraform", "env"
	Sensitive        types.Bool   `tfsdk:"sensitive"`         // Whether the variable contains sensitive data
	HCL              types.Bool   `tfsdk:"hcl"`               // Whether to parse the value as HCL
	CreatedAt        types.String `tfsdk:"created_at"`        // Timestamp
	UpdatedAt        types.String `tfsdk:"updated_at"`        // Timestamp
}

// VariableAPIResponse represents the JSON structure returned by the API
type VariableAPIResponse struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Sensitive   bool      `json:"sensitive"`
	HCL         bool      `json:"hcl"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VariableCreateRequest represents the JSON structure for creating a variable
type VariableCreateRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	HCL         bool   `json:"hcl,omitempty"`
}

// VariableUpdateRequest represents the JSON structure for updating a variable
type VariableUpdateRequest struct {
	Key         string `json:"key,omitempty"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Sensitive   *bool  `json:"sensitive,omitempty"`
	HCL         *bool  `json:"hcl,omitempty"`
}

type VariableResource struct {
	provider *InfradotsProvider
}

func (r *VariableResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_variable"
}

func (r *VariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The variable unique ID (UUID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this variable belongs to.",
				Required:    true,
			},
			"key": schema.StringAttribute{
				Description: "The name of the variable.",
				Required:    true,
			},
			"value": schema.StringAttribute{
				Description: "The value of the variable.",
				Required:    true,
				Sensitive:   true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the variable.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"category": schema.StringAttribute{
				Description: "The category of the variable. Valid values are 'terraform' or 'env'.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("terraform"),
			},
			"sensitive": schema.BoolAttribute{
				Description: "Whether the variable contains sensitive information.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"hcl": schema.BoolAttribute{
				Description: "Whether to parse the value as HCL.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the variable was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the variable was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *VariableResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *VariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VariableResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare the request
	createReq := VariableCreateRequest{
		Key:         data.Key.ValueString(),
		Value:       data.Value.ValueString(),
		Description: data.Description.ValueString(),
		Category:    data.Category.ValueString(),
		Sensitive:   data.Sensitive.ValueBool(),
		HCL:         data.HCL.ValueBool(),
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	// POST to /api/organizations/{organization_name}/variables/
	url := fmt.Sprintf("http://%s/api/organizations/%s/variables/",
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

	// Read the response body
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

	// Parse the response
	var variable VariableAPIResponse
	err = json.Unmarshal(respBody, &variable)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Update the model with the response data
	data.ID = types.StringValue(variable.ID)
	data.Key = types.StringValue(variable.Key)
	data.Value = types.StringValue(variable.Value)
	data.Description = types.StringValue(variable.Description)
	data.Category = types.StringValue(variable.Category)
	data.Sensitive = types.BoolValue(variable.Sensitive)
	data.HCL = types.BoolValue(variable.HCL)
	data.CreatedAt = types.StringValue(variable.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(variable.UpdatedAt.Format(time.RFC3339))

	// Save data back into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *VariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VariableResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// GET from /api/organizations/{organization_name}/variables/{variable_id}/
	url := fmt.Sprintf("http://%s/api/organizations/%s/variables/%s/",
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

	// If 404, resource no longer exists
	if httpResp.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
		return
	}

	// Read and parse the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var variable VariableAPIResponse
	err = json.Unmarshal(respBody, &variable)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Update the model with the response data
	data.ID = types.StringValue(variable.ID)
	data.Key = types.StringValue(variable.Key)
	data.Value = types.StringValue(variable.Value)
	data.Description = types.StringValue(variable.Description)
	data.Category = types.StringValue(variable.Category)
	data.Sensitive = types.BoolValue(variable.Sensitive)
	data.HCL = types.BoolValue(variable.HCL)
	data.CreatedAt = types.StringValue(variable.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(variable.UpdatedAt.Format(time.RFC3339))

	// Save (possibly updated) state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *VariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state VariableResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan VariableResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare the update request with only the fields that are changing
	updateReq := VariableUpdateRequest{}

	if !plan.Key.Equal(state.Key) {
		updateReq.Key = plan.Key.ValueString()
	}

	if !plan.Value.Equal(state.Value) {
		updateReq.Value = plan.Value.ValueString()
	}

	if !plan.Description.Equal(state.Description) {
		updateReq.Description = plan.Description.ValueString()
	}

	if !plan.Category.Equal(state.Category) {
		updateReq.Category = plan.Category.ValueString()
	}

	if !plan.Sensitive.Equal(state.Sensitive) {
		sensitive := plan.Sensitive.ValueBool()
		updateReq.Sensitive = &sensitive
	}

	if !plan.HCL.Equal(state.HCL) {
		hcl := plan.HCL.ValueBool()
		updateReq.HCL = &hcl
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	// PATCH to /api/organizations/{organization_name}/variables/{variable_id}/
	url := fmt.Sprintf("http://%s/api/organizations/%s/variables/%s/",
		r.provider.host,
		plan.OrganizationName.ValueString(),
		plan.ID.ValueString())

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

	// Read the response body
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

	// Parse the response
	var variable VariableAPIResponse
	err = json.Unmarshal(respBody, &variable)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Update the model with the response data
	plan.ID = types.StringValue(variable.ID)
	plan.Key = types.StringValue(variable.Key)
	plan.Value = types.StringValue(variable.Value)
	plan.Description = types.StringValue(variable.Description)
	plan.Category = types.StringValue(variable.Category)
	plan.Sensitive = types.BoolValue(variable.Sensitive)
	plan.HCL = types.BoolValue(variable.HCL)
	plan.CreatedAt = types.StringValue(variable.CreatedAt.Format(time.RFC3339))
	plan.UpdatedAt = types.StringValue(variable.UpdatedAt.Format(time.RFC3339))

	// Save updated info
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *VariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VariableResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DELETE from /api/organizations/{organization_name}/variables/{variable_id}/
	url := fmt.Sprintf("http://%s/api/organizations/%s/variables/%s/",
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

	// Read the response body for error details if needed
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

	// Remove resource from state
	resp.State.RemoveResource(ctx)
}

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tflog "github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure we fully satisfy the resource.Resource interface.
var _ resource.Resource = &VCSResource{}

func NewVCSResource() resource.Resource {
	return &VCSResource{}
}

// VCSResourceModel maps the VCS resource schema data.
type VCSResourceModel struct {
	ID               types.String `tfsdk:"id"`                // UUID
	OrganizationName types.String `tfsdk:"organization_name"` // Name of the organization
	Name             types.String `tfsdk:"name"`              // VCS name
	VcsType          types.String `tfsdk:"vcs_type"`          // VCS type (e.g., "github", "gitlab", "bitbucket")
	URL              types.String `tfsdk:"url"`               // VCS URL
	ClientId         types.String `tfsdk:"client_id"`         // VCS Client ID
	ClientSecret     types.String `tfsdk:"client_secret"`     // VCS Client Secret
	Description      types.String `tfsdk:"description"`       // Optional description
	CreatedAt        types.String `tfsdk:"created_at"`        // Timestamp
	UpdatedAt        types.String `tfsdk:"updated_at"`        // Timestamp
	ConnectionType   types.String `tfsdk:"connection_type"`   // OAUTH or SSH
	PrivateKey       types.String `tfsdk:"private_key"`       // SSH private key (write-only)
	Endpoint         types.String `tfsdk:"endpoint"`          // Base URL for self-hosted VCS
	ApiUrl           types.String `tfsdk:"api_url"`           // API URL for self-hosted VCS
}

// VCSAPIResponse represents the JSON structure returned by the API
type VCSAPIResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	VcsType        string    `json:"vcsType"`
	URL            string    `json:"endpoint"`
	ClientId       string    `json:"clientId"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ConnectionType string    `json:"connectionType"`
	EndpointUrl    string    `json:"endpointUrl"`
	ApiUrl         string    `json:"apiUrl"`
}

// VCSCreateRequest represents the JSON structure for creating a VCS
type VCSCreateRequest struct {
	Name           string `json:"name"`
	VcsType        string `json:"vcsType"`
	URL            string `json:"endpoint"`
	ClientId       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	Description    string `json:"description,omitempty"`
	ConnectionType string `json:"connectionType,omitempty"`
	PrivateKey     string `json:"privateKey,omitempty"`
	EndpointUrl    string `json:"endpointUrl,omitempty"`
	ApiUrl         string `json:"apiUrl,omitempty"`
}

// VCSUpdateRequest represents the JSON structure for updating a VCS
type VCSUpdateRequest struct {
	Name           string `json:"name,omitempty"`
	VcsType        string `json:"vcsType,omitempty"`
	URL            string `json:"endpoint,omitempty"`
	ClientId       string `json:"clientId,omitempty"`
	ClientSecret   string `json:"clientSecret,omitempty"`
	Description    string `json:"description,omitempty"`
	ConnectionType string `json:"connectionType,omitempty"`
	PrivateKey     string `json:"privateKey,omitempty"`
	EndpointUrl    string `json:"endpointUrl,omitempty"`
	ApiUrl         string `json:"apiUrl,omitempty"`
}

type VCSResource struct {
	provider *InfradotsProvider
}

func (r *VCSResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_vcs"
}

func (r *VCSResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The VCS unique ID (UUID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this VCS belongs to.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the VCS connection.",
				Required:    true,
			},
			"vcs_type": schema.StringAttribute{
				Description: "The type of VCS (e.g., github, gitlab, bitbucket).",
				Required:    true,
			},
			"url": schema.StringAttribute{
				Description: "The URL of the VCS instance.",
				Required:    true,
			},
			"client_id": schema.StringAttribute{
				Description: "The client ID for the VCS.",
				Required:    true,
				Sensitive:   false,
			},
			"client_secret": schema.StringAttribute{
				Description: "The client secret token for the VCS.",
				Required:    true,
				Sensitive:   true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the VCS connection.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the VCS was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the VCS was last updated.",
				Computed:    true,
			},
			"connection_type": schema.StringAttribute{
				Description: "Connection type for the VCS: OAUTH or SSH.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("OAUTH"),
				Validators: []validator.String{
					stringvalidator.OneOf("OAUTH", "SSH"),
				},
			},
			"private_key": schema.StringAttribute{
				Description: "Private key for SSH-based VCS authentication.",
				Optional:    true,
				Sensitive:   true,
			},
			"endpoint": schema.StringAttribute{
				Description: "Base URL for self-hosted VCS instances.",
				Optional:    true,
				Computed:    true,
			},
			"api_url": schema.StringAttribute{
				Description: "API URL for self-hosted VCS instances.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *VCSResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *VCSResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VCSResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := VCSCreateRequest{
		Name:           data.Name.ValueString(),
		VcsType:        data.VcsType.ValueString(),
		URL:            data.URL.ValueString(),
		ClientId:       data.ClientId.ValueString(),
		ClientSecret:   data.ClientSecret.ValueString(),
		Description:    data.Description.ValueString(),
		ConnectionType: data.ConnectionType.ValueString(),
	}
	if !data.PrivateKey.IsNull() {
		createReq.PrivateKey = data.PrivateKey.ValueString()
	}
	if !data.Endpoint.IsNull() {
		createReq.EndpointUrl = data.Endpoint.ValueString()
	}
	if !data.ApiUrl.IsNull() {
		createReq.ApiUrl = data.ApiUrl.ValueString()
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/vcs/",
		r.provider.host,
		data.OrganizationName.ValueString())
	tflog.Debug(ctx, url)
	reqHttp, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	tflog.Debug(ctx, "HTTP request", map[string]any{
		"method": reqHttp.Method,
		"url":    reqHttp.URL.String(), // or req.URL.Redacted()
	})
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

	var vcs VCSAPIResponse
	err = json.Unmarshal(respBody, &vcs)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(vcs.ID)
	data.Name = types.StringValue(vcs.Name)
	data.VcsType = types.StringValue(vcs.VcsType)
	data.URL = types.StringValue(vcs.URL)
	data.ClientId = types.StringValue(vcs.ClientId)
	// clientSecret is write-only, not returned by API, keep existing value in state
	data.Description = types.StringValue(vcs.Description)
	data.CreatedAt = types.StringValue(vcs.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(vcs.UpdatedAt.Format(time.RFC3339))
	data.ConnectionType = types.StringValue(vcs.ConnectionType)
	// private_key is write-only, keep existing value in state
	data.Endpoint = types.StringValue(vcs.EndpointUrl)
	data.ApiUrl = types.StringValue(vcs.ApiUrl)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *VCSResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VCSResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/vcs/%s/",
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

	var vcs VCSAPIResponse
	err = json.Unmarshal(respBody, &vcs)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Update the model with the response data
	data.ID = types.StringValue(vcs.ID)
	data.Name = types.StringValue(vcs.Name)
	data.VcsType = types.StringValue(vcs.VcsType)
	data.URL = types.StringValue(vcs.URL)
	data.ClientId = types.StringValue(vcs.ClientId)
	// clientSecret is write-only, not returned by API, keep existing value in state
	data.Description = types.StringValue(vcs.Description)
	data.CreatedAt = types.StringValue(vcs.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(vcs.UpdatedAt.Format(time.RFC3339))
	data.ConnectionType = types.StringValue(vcs.ConnectionType)
	// private_key is write-only, keep existing value in state
	data.Endpoint = types.StringValue(vcs.EndpointUrl)
	data.ApiUrl = types.StringValue(vcs.ApiUrl)

	// Save (possibly updated) state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *VCSResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state VCSResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan VCSResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare the update request with only the fields that are changing
	updateReq := VCSUpdateRequest{}

	if !plan.Name.Equal(state.Name) {
		updateReq.Name = plan.Name.ValueString()
	}

	if !plan.VcsType.Equal(state.VcsType) {
		updateReq.VcsType = plan.VcsType.ValueString()
	}

	if !plan.URL.Equal(state.URL) {
		updateReq.URL = plan.URL.ValueString()
	}

	if !plan.ClientId.Equal(state.ClientId) {
		updateReq.ClientId = plan.ClientId.ValueString()
	}
	if !plan.ClientSecret.Equal(state.ClientSecret) {
		updateReq.ClientSecret = plan.ClientSecret.ValueString()
	}

	if !plan.Description.Equal(state.Description) {
		updateReq.Description = plan.Description.ValueString()
	}

	if !plan.ConnectionType.Equal(state.ConnectionType) {
		updateReq.ConnectionType = plan.ConnectionType.ValueString()
	}

	if !plan.PrivateKey.Equal(state.PrivateKey) && !plan.PrivateKey.IsNull() {
		updateReq.PrivateKey = plan.PrivateKey.ValueString()
	}

	if !plan.Endpoint.Equal(state.Endpoint) && !plan.Endpoint.IsNull() {
		updateReq.EndpointUrl = plan.Endpoint.ValueString()
	}

	if !plan.ApiUrl.Equal(state.ApiUrl) && !plan.ApiUrl.IsNull() {
		updateReq.ApiUrl = plan.ApiUrl.ValueString()
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	// PATCH to /api/organizations/{organization_name}/vcs/{vcs_id}/
	url := fmt.Sprintf("https://%s/api/organizations/%s/vcs/%s/",
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
	var vcs VCSAPIResponse
	err = json.Unmarshal(respBody, &vcs)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Update the model with the response data
	plan.ID = types.StringValue(vcs.ID)
	plan.Name = types.StringValue(vcs.Name)
	plan.VcsType = types.StringValue(vcs.VcsType)
	plan.URL = types.StringValue(vcs.URL)
	plan.ClientId = types.StringValue(vcs.ClientId)
	// clientSecret is write-only, not returned by API, keep value from plan
	plan.Description = types.StringValue(vcs.Description)
	plan.CreatedAt = types.StringValue(vcs.CreatedAt.Format(time.RFC3339))
	plan.UpdatedAt = types.StringValue(vcs.UpdatedAt.Format(time.RFC3339))
	plan.ConnectionType = types.StringValue(vcs.ConnectionType)
	// private_key is write-only, keep value from plan
	plan.Endpoint = types.StringValue(vcs.EndpointUrl)
	plan.ApiUrl = types.StringValue(vcs.ApiUrl)

	// Save updated info
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *VCSResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VCSResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DELETE from /api/organizations/{organization_name}/vcs/{vcs_id}/
	url := fmt.Sprintf("https://%s/api/organizations/%s/vcs/%s/",
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

func (r *VCSResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Parse the import ID: format is "organization_name:vcs_name"
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:vcs_name'",
		)
		return
	}

	organizationName := parts[0]
	vcsName := parts[1]

	// Use the list endpoint and filter by name (same approach as datasource)
	url := fmt.Sprintf("https://%s/api/organizations/%s/vcs/",
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
			"Failed to fetch VCS connections",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	// Read and parse the response body (list of VCS connections)
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var vcsList []VCSAPIResponse
	err = json.Unmarshal(respBody, &vcsList)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Find the VCS by name
	var vcs *VCSAPIResponse
	found := false
	for i := range vcsList {
		if vcsList[i].Name == vcsName {
			vcs = &vcsList[i]
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"VCS connection not found",
			fmt.Sprintf("No VCS connection with name '%s' found in organization '%s'", vcsName, organizationName),
		)
		return
	}

	// Create the state model with the fetched data
	var data VCSResourceModel
	data.ID = types.StringValue(vcs.ID)
	data.OrganizationName = types.StringValue(organizationName)
	data.Name = types.StringValue(vcs.Name)
	data.VcsType = types.StringValue(vcs.VcsType)
	data.URL = types.StringValue(vcs.URL)
	data.ClientId = types.StringValue(vcs.ClientId)
	// clientSecret is write-only, not returned by API - will need to be set manually
	data.Description = types.StringValue(vcs.Description)
	data.CreatedAt = types.StringValue(vcs.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(vcs.UpdatedAt.Format(time.RFC3339))
	data.ConnectionType = types.StringValue(vcs.ConnectionType)
	// private_key is write-only, not returned by API - will need to be set manually
	data.Endpoint = types.StringValue(vcs.EndpointUrl)
	data.ApiUrl = types.StringValue(vcs.ApiUrl)

	// Set the state
	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

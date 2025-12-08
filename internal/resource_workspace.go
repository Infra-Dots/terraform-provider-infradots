package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure we fully satisfy the resource.Resource interface.
var _ resource.Resource = &WorkspaceResource{}

func NewWorkspaceResource() resource.Resource {
	return &WorkspaceResource{}
}

type WorkspaceResourceModel struct {
	ID               types.String `tfsdk:"id"`                // UUID
	OrganizationName types.String `tfsdk:"organization_name"` // Name of the organization
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	Source           types.String `tfsdk:"source"`
	Branch           types.String `tfsdk:"branch"`
	TerraformVersion types.String `tfsdk:"terraform_version"`
	CreatedAt        types.String `tfsdk:"created_at"` // timestamp
	UpdatedAt        types.String `tfsdk:"updated_at"` // timestamp
	VcsId            types.String `tfsdk:"vcs_id"`     // UUID of a VCS provider in IDP
	VCS              types.Object `tfsdk:"vcs"`        // VCS object as returned by API
}

type WorkspaceAPIResponse struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Source           string          `json:"source"`
	Branch           string          `json:"branch"`
	TerraformVersion string          `json:"terraform_version"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	VCS              *VCSAPIResponse `json:"vcs"` // VCS object from API
}

// WorkspaceCreateRequest represents the JSON structure for creating a workspace
type WorkspaceCreateRequest struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Source           string `json:"source"`
	Branch           string `json:"branch"`
	TerraformVersion string `json:"terraform_version"`
}

// WorkspaceUpdateRequest represents the JSON structure for updating a workspace
type WorkspaceUpdateRequest struct {
	Name             string `json:"name,omitempty"`
	Description      string `json:"description,omitempty"`
	Source           string `json:"source,omitempty"`
	Branch           string `json:"branch,omitempty"`
	TerraformVersion string `json:"terraform_version,omitempty"`
}

type WorkspaceResource struct {
	provider *InfradotsProvider
}

func (r *WorkspaceResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_workspace"
}

func (r *WorkspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The workspace unique ID (UUID).",
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this workspace belongs to.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the workspace.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "A short description of the workspace.",
				Optional:    true,
			},
			"source": schema.StringAttribute{
				Description: "Source repository URL or path.",
				Required:    true,
			},
			"branch": schema.StringAttribute{
				Description: "Git branch to use (if applicable).",
				Required:    true,
			},
			"terraform_version": schema.StringAttribute{
				Description: "Terraform version to use.",
				Required:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the workspace was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the workspace was last updated.",
				Computed:    true,
			},
			"vcs_id": schema.StringAttribute{
				Description: "ID of a VCS Provider in infradots to connect to the workspace",
				Required:    false,
				Optional:    true,
			},
			"vcs": schema.SingleNestedAttribute{
				Description: "VCS connection details associated with this workspace.",
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Description: "The VCS unique ID (UUID).",
						Computed:    true,
					},
					"name": schema.StringAttribute{
						Description: "The name of the VCS connection.",
						Computed:    true,
					},
					"vcs_type": schema.StringAttribute{
						Description: "The type of VCS (e.g., github, gitlab, bitbucket).",
						Computed:    true,
					},
					"url": schema.StringAttribute{
						Description: "The URL of the VCS instance.",
						Computed:    true,
					},
					"description": schema.StringAttribute{
						Description: "A description of the VCS connection.",
						Computed:    true,
					},
					"created_at": schema.StringAttribute{
						Description: "The timestamp when the VCS was created.",
						Computed:    true,
					},
					"updated_at": schema.StringAttribute{
						Description: "The timestamp when the VCS was last updated.",
						Computed:    true,
					},
				},
			},
		},
	}
}

func (r *WorkspaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

// Helper function to convert VCS API response to types.Object
func vcsToObject(vcs *VCSAPIResponse) types.Object {
	if vcs == nil {
		return types.ObjectNull(map[string]attr.Type{
			"id":          types.StringType,
			"name":        types.StringType,
			"vcs_type":    types.StringType,
			"url":         types.StringType,
			"description": types.StringType,
			"created_at":  types.StringType,
			"updated_at":  types.StringType,
		})
	}

	return types.ObjectValueMust(
		map[string]attr.Type{
			"id":          types.StringType,
			"name":        types.StringType,
			"vcs_type":    types.StringType,
			"url":         types.StringType,
			"description": types.StringType,
			"created_at":  types.StringType,
			"updated_at":  types.StringType,
		},
		map[string]attr.Value{
			"id":          types.StringValue(vcs.ID),
			"name":        types.StringValue(vcs.Name),
			"vcs_type":    types.StringValue(vcs.VcsType),
			"url":         types.StringValue(vcs.URL),
			"description": types.StringValue(vcs.Description),
			"created_at":  types.StringValue(vcs.CreatedAt.Format(time.RFC3339)),
			"updated_at":  types.StringValue(vcs.UpdatedAt.Format(time.RFC3339)),
		},
	)
}

func (r *WorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare the request
	createReq := WorkspaceCreateRequest{
		Name:             data.Name.ValueString(),
		Description:      data.Description.ValueString(),
		Source:           data.Source.ValueString(),
		Branch:           data.Branch.ValueString(),
		TerraformVersion: data.TerraformVersion.ValueString(),
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	// POST to /api/organizations/{organization_name}/workspaces/
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/",
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
	var workspace WorkspaceAPIResponse
	err = json.Unmarshal(respBody, &workspace)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Update the model with the response data
	data.ID = types.StringValue(workspace.ID)
	data.Name = types.StringValue(workspace.Name)
	data.Description = types.StringValue(workspace.Description)
	data.Source = types.StringValue(workspace.Source)
	data.Branch = types.StringValue(workspace.Branch)
	data.TerraformVersion = types.StringValue(workspace.TerraformVersion)
	data.CreatedAt = types.StringValue(workspace.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(workspace.UpdatedAt.Format(time.RFC3339))
	data.VCS = vcsToObject(workspace.VCS)

	// Save data back into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// GET from /api/organizations/{organization_name}/workspaces/{workspace_name}/
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.Name.ValueString())

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

	var workspace WorkspaceAPIResponse
	err = json.Unmarshal(respBody, &workspace)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Update the model with the response data
	data.ID = types.StringValue(workspace.ID)
	data.Name = types.StringValue(workspace.Name)
	data.Description = types.StringValue(workspace.Description)
	data.Source = types.StringValue(workspace.Source)
	data.Branch = types.StringValue(workspace.Branch)
	data.TerraformVersion = types.StringValue(workspace.TerraformVersion)
	data.CreatedAt = types.StringValue(workspace.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(workspace.UpdatedAt.Format(time.RFC3339))
	data.VCS = vcsToObject(workspace.VCS)

	// Save (possibly updated) state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state WorkspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WorkspaceResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare the update request with only the fields that are changing
	updateReq := WorkspaceUpdateRequest{}

	if !plan.Name.Equal(state.Name) {
		updateReq.Name = plan.Name.ValueString()
	}

	if !plan.Description.Equal(state.Description) {
		updateReq.Description = plan.Description.ValueString()
	}

	if !plan.Source.Equal(state.Source) {
		updateReq.Source = plan.Source.ValueString()
	}

	if !plan.Branch.Equal(state.Branch) {
		updateReq.Branch = plan.Branch.ValueString()
	}

	if !plan.TerraformVersion.Equal(state.TerraformVersion) {
		updateReq.TerraformVersion = plan.TerraformVersion.ValueString()
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	// PATCH to /api/organizations/{organization_name}/workspaces/{workspace_name}/
	// Use state name (current name) for the URL, not plan name (which might be changing)
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/",
		r.provider.host,
		plan.OrganizationName.ValueString(),
		state.Name.ValueString())

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
	var workspace WorkspaceAPIResponse
	err = json.Unmarshal(respBody, &workspace)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Update the model with the response data
	plan.ID = types.StringValue(workspace.ID)
	plan.Name = types.StringValue(workspace.Name)
	plan.Description = types.StringValue(workspace.Description)
	plan.Source = types.StringValue(workspace.Source)
	plan.Branch = types.StringValue(workspace.Branch)
	plan.TerraformVersion = types.StringValue(workspace.TerraformVersion)
	plan.CreatedAt = types.StringValue(workspace.CreatedAt.Format(time.RFC3339))
	plan.UpdatedAt = types.StringValue(workspace.UpdatedAt.Format(time.RFC3339))
	plan.VCS = vcsToObject(workspace.VCS)

	// Save updated info
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DELETE from /api/organizations/{organization_name}/workspaces/{workspace_name}/
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.Name.ValueString())

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

func (r *WorkspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Parse the import ID: format is "organization_name:workspace_name"
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

	// Use the list endpoint and filter by name (same approach as datasource)
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/",
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
			"Failed to fetch workspaces",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	// Read and parse the response body (list of workspaces)
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var workspaces []WorkspaceAPIResponse
	err = json.Unmarshal(respBody, &workspaces)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Find the workspace by name
	var workspace *WorkspaceAPIResponse
	found := false
	for i := range workspaces {
		if workspaces[i].Name == workspaceName {
			workspace = &workspaces[i]
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Workspace not found",
			fmt.Sprintf("No workspace with name '%s' found in organization '%s'", workspaceName, organizationName),
		)
		return
	}

	// Create the state model with the fetched data
	// Match the exact pattern used in Read method
	var data WorkspaceResourceModel
	data.ID = types.StringValue(workspace.ID)
	data.OrganizationName = types.StringValue(organizationName)
	data.Name = types.StringValue(workspace.Name)
	data.Description = types.StringValue(workspace.Description)
	data.Source = types.StringValue(workspace.Source)
	data.Branch = types.StringValue(workspace.Branch)
	data.TerraformVersion = types.StringValue(workspace.TerraformVersion)
	data.CreatedAt = types.StringValue(workspace.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(workspace.UpdatedAt.Format(time.RFC3339))
	data.VCS = vcsToObject(workspace.VCS)
	// Note: VcsId is not set here, matching the Read method pattern
	// VcsId is only used for input during create/update operations

	// Set the state
	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

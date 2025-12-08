package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &WorkspaceDataSource{}

func NewWorkspaceDataSource() datasource.DataSource {
	return &WorkspaceDataSource{}
}

type WorkspaceDataSource struct {
	provider *InfradotsProvider
}

type WorkspaceDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	Source           types.String `tfsdk:"source"`
	Branch           types.String `tfsdk:"branch"`
	TerraformVersion types.String `tfsdk:"terraform_version"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
	VCS              types.Object `tfsdk:"vcs"`
}

type WorkspaceDataSourceFilterModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
}

func (d *WorkspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_data"
}

func (d *WorkspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a workspace by ID or by organization name and workspace name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the workspace.",
				Optional:    true,
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this workspace belongs to.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the workspace.",
				Optional:    true,
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "A short description of the workspace.",
				Computed:    true,
			},
			"source": schema.StringAttribute{
				Description: "Source repository URL or path.",
				Computed:    true,
			},
			"branch": schema.StringAttribute{
				Description: "Git branch to use (if applicable).",
				Computed:    true,
			},
			"terraform_version": schema.StringAttribute{
				Description: "Terraform version to use.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the workspace was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the workspace was last updated.",
				Computed:    true,
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

func (d *WorkspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*InfradotsProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *InfradotsProvider, got: %T", req.ProviderData),
		)
		return
	}

	d.provider = provider
}

// Helper function to convert VCS API response to types.Object for data source
func vcsToObjectDataSource(vcs *VCSAPIResponse) types.Object {
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

func (d *WorkspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceDataSourceModel
	var filter WorkspaceDataSourceFilterModel

	// Read input configuration into filter
	resp.Diagnostics.Append(req.Config.Get(ctx, &filter)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate input parameters
	if filter.ID.IsNull() && (filter.OrganizationName.IsNull() || filter.Name.IsNull()) {
		resp.Diagnostics.AddError(
			"Missing required parameter",
			"Either id or both organization_name and name must be specified",
		)
		return
	}

	var url string
	// Determine the URL based on the filter
	if !filter.ID.IsNull() {
		// We need to first determine the organization name for this workspace ID
		// This would typically require an additional API call to get the workspace details
		// For this implementation, we'll require organization_name to be provided alongside ID
		if filter.OrganizationName.IsNull() {
			resp.Diagnostics.AddError(
				"Missing required parameter",
				"When filtering by ID, organization_name must also be specified",
			)
			return
		}
		url = fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/",
			d.provider.host,
			filter.OrganizationName.ValueString(),
			filter.ID.ValueString())
	} else {
		// Fetch by organization name and workspace name
		url = fmt.Sprintf("https://%s/api/organizations/%s/workspaces/",
			d.provider.host,
			filter.OrganizationName.ValueString())
	}

	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.provider.token)

	// Execute the request
	httpResp, err := d.provider.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error making HTTP request", err.Error())
		return
	}
	defer httpResp.Body.Close()

	// Handle non-successful response
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Unexpected HTTP status code",
			fmt.Sprintf("Expected 200, got: %d", httpResp.StatusCode),
		)
		return
	}

	// Read the response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	// Process based on filter type
	if !filter.ID.IsNull() {
		// Single workspace response
		var apiResp WorkspaceAPIResponse
		err = json.Unmarshal(body, &apiResp)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		// Map response to model
		data.ID = types.StringValue(apiResp.ID)
		data.OrganizationName = filter.OrganizationName
		data.Name = types.StringValue(apiResp.Name)
		data.Description = types.StringValue(apiResp.Description)
		data.Source = types.StringValue(apiResp.Source)
		data.Branch = types.StringValue(apiResp.Branch)
		data.TerraformVersion = types.StringValue(apiResp.TerraformVersion)
		data.CreatedAt = types.StringValue(apiResp.CreatedAt.Format(time.RFC3339))
		data.UpdatedAt = types.StringValue(apiResp.UpdatedAt.Format(time.RFC3339))
		data.VCS = vcsToObjectDataSource(apiResp.VCS)
	} else {
		// List of workspaces, filter by name
		var apiRespList []WorkspaceAPIResponse
		err = json.Unmarshal(body, &apiRespList)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		found := false
		for _, workspace := range apiRespList {
			if workspace.Name == filter.Name.ValueString() {
				// Map response to model
				data.ID = types.StringValue(workspace.ID)
				data.OrganizationName = filter.OrganizationName
				data.Name = types.StringValue(workspace.Name)
				data.Description = types.StringValue(workspace.Description)
				data.Source = types.StringValue(workspace.Source)
				data.Branch = types.StringValue(workspace.Branch)
				data.TerraformVersion = types.StringValue(workspace.TerraformVersion)
				data.CreatedAt = types.StringValue(workspace.CreatedAt.Format(time.RFC3339))
				data.UpdatedAt = types.StringValue(workspace.UpdatedAt.Format(time.RFC3339))
				data.VCS = vcsToObjectDataSource(workspace.VCS)
				found = true
				break
			}
		}

		if !found {
			resp.Diagnostics.AddError(
				"Workspace not found",
				fmt.Sprintf("No workspace with name '%s' found in organization '%s'",
					filter.Name.ValueString(),
					filter.OrganizationName.ValueString()),
			)
			return
		}
	}

	// Set the data for the response
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

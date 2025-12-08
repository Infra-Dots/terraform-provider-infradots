package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var _ datasource.DataSource = &VCSDataSource{}

// NewVCSDataSource is a helper function to simplify the provider implementation.
func NewVCSDataSource() datasource.DataSource {
	return &VCSDataSource{}
}

// VCSDataSource is the data source implementation.
type VCSDataSource struct {
	provider *InfradotsProvider
}

// VCSDataSourceModel maps the data source schema data.
type VCSDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	VcsType          types.String `tfsdk:"vcs_type"`
	URL              types.String `tfsdk:"url"`
	ClientId         types.String `tfsdk:"client_id"`
	Description      types.String `tfsdk:"description"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

// VCSDataSourceFilterModel maps the filter parameters.
type VCSDataSourceFilterModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
}

// Metadata returns the data source type name.
func (d *VCSDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vcs_data"
}

// Schema defines the schema for the data source.
func (d *VCSDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a VCS connection by ID or by organization name and VCS name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the VCS connection.",
				Optional:    true,
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this VCS connection belongs to.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the VCS connection.",
				Optional:    true,
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
			"client_id": schema.StringAttribute{
				Description: "The client ID for the VCS.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the VCS connection.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the VCS connection was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the VCS connection was last updated.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *VCSDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read fetches the data from the API.
func (d *VCSDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VCSDataSourceModel
	var filter VCSDataSourceFilterModel

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
		// We need to first determine the organization name for this VCS ID
		// This would typically require an additional API call to get the VCS details
		// For this implementation, we'll require organization_name to be provided alongside ID
		if filter.OrganizationName.IsNull() {
			resp.Diagnostics.AddError(
				"Missing required parameter",
				"When filtering by ID, organization_name must also be specified",
			)
			return
		}
		url = fmt.Sprintf("https://%s/api/organizations/%s/vcs/%s/",
			d.provider.host,
			filter.OrganizationName.ValueString(),
			filter.ID.ValueString())
	} else {
		// Fetch by organization name and VCS name
		// First get all VCS connections for the organization, then filter by name
		url = fmt.Sprintf("https://%s/api/organizations/%s/vcs/",
			d.provider.host,
			filter.OrganizationName.ValueString())
	}

	// Create HTTP request
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
		// Single VCS response
		var apiResp VCSAPIResponse
		err = json.Unmarshal(body, &apiResp)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		// Map response to model
		data.ID = types.StringValue(apiResp.ID)
		data.OrganizationName = filter.OrganizationName
		data.Name = types.StringValue(apiResp.Name)
		data.VcsType = types.StringValue(apiResp.VcsType)
		data.URL = types.StringValue(apiResp.URL)
		data.ClientId = types.StringValue(apiResp.ClientId)
		data.Description = types.StringValue(apiResp.Description)
		data.CreatedAt = types.StringValue(apiResp.CreatedAt.Format(time.RFC3339))
		data.UpdatedAt = types.StringValue(apiResp.UpdatedAt.Format(time.RFC3339))
	} else {
		// List of VCS connections, filter by name
		var apiRespList []VCSAPIResponse
		err = json.Unmarshal(body, &apiRespList)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		found := false
		for _, vcs := range apiRespList {
			if vcs.Name == filter.Name.ValueString() {
				// Map response to model
				data.ID = types.StringValue(vcs.ID)
				data.OrganizationName = filter.OrganizationName
				data.Name = types.StringValue(vcs.Name)
				data.VcsType = types.StringValue(vcs.VcsType)
				data.URL = types.StringValue(vcs.URL)
				data.ClientId = types.StringValue(vcs.ClientId)
				data.Description = types.StringValue(vcs.Description)
				data.CreatedAt = types.StringValue(vcs.CreatedAt.Format(time.RFC3339))
				data.UpdatedAt = types.StringValue(vcs.UpdatedAt.Format(time.RFC3339))
				found = true
				break
			}
		}

		if !found {
			resp.Diagnostics.AddError(
				"VCS connection not found",
				fmt.Sprintf("No VCS connection with name '%s' found in organization '%s'",
					filter.Name.ValueString(),
					filter.OrganizationName.ValueString()),
			)
			return
		}
	}

	// Set the data for the response
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

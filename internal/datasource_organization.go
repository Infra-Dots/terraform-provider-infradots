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

// Ensure the implementation satisfies the expected interfaces.
var _ datasource.DataSource = &OrganizationDataSource{}

// NewOrganizationDataSource is a helper function to simplify the provider implementation.
func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

// OrganizationDataSource is the data source implementation.
type OrganizationDataSource struct {
	provider *InfradotsProvider
}

// OrganizationDataSourceModel maps the data source schema data.
type OrganizationDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
	ExecutionMode types.String `tfsdk:"execution_mode"`
	AgentsEnabled types.Bool   `tfsdk:"agents_enabled"`
	Members       types.List   `tfsdk:"members"`
	Teams         types.List   `tfsdk:"teams"`
}

// OrganizationDataSourceFilterModel maps the filter parameters.
type OrganizationDataSourceFilterModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// OrganizationMemberModel maps the member data.
type OrganizationMemberModel struct {
	Email types.String `tfsdk:"email"`
}

// OrganizationTeamModel maps the team data.
type OrganizationTeamModel struct {
	Name types.String `tfsdk:"name"`
}

// Metadata returns the data source type name.
func (d *OrganizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_data"
}

// Schema defines the schema for the data source.
func (d *OrganizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an organization by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the organization.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the organization.",
				Optional:    true,
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the organization was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the organization was last updated.",
				Computed:    true,
			},
			"execution_mode": schema.StringAttribute{
				Description: "The execution mode for the organization (Remote, Local, etc.).",
				Computed:    true,
			},
			"agents_enabled": schema.BoolAttribute{
				Description: "Whether agents are enabled for the organization.",
				Computed:    true,
			},
			"members": schema.ListNestedAttribute{
				Description: "The members of the organization.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"email": schema.StringAttribute{
							Description: "The email address of the member.",
							Computed:    true,
						},
					},
				},
			},
			"teams": schema.ListNestedAttribute{
				Description: "The teams in the organization.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the team.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *OrganizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationDataSourceModel
	var filter OrganizationDataSourceFilterModel

	// Read input configuration into filter
	resp.Diagnostics.Append(req.Config.Get(ctx, &filter)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate input parameters
	if filter.ID.IsNull() && filter.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing required parameter",
			"Either id or name must be specified",
		)
		return
	}

	// Determine the URL based on the filter
	var url string
	if !filter.ID.IsNull() {
		// Fetch by ID
		url = fmt.Sprintf("https://%s/api/organizations/%s/", d.provider.host, filter.ID.ValueString())
	} else {
		// Fetch by name (first get all, then filter)
		url = fmt.Sprintf("https://%s/api/organizations/", d.provider.host)
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
		// Single organization response
		var apiResp OrganizationAPIResponse
		err = json.Unmarshal(body, &apiResp)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}
		d.mapOrganizationToModel(ctx, &data, apiResp, resp)
	} else {
		// List of organizations, filter by name
		var apiRespList []OrganizationAPIResponse
		err = json.Unmarshal(body, &apiRespList)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		found := false
		for _, org := range apiRespList {
			if org.Name == filter.Name.ValueString() {
				d.mapOrganizationToModel(ctx, &data, org, resp)
				found = true
				break
			}
		}

		if !found {
			resp.Diagnostics.AddError(
				"Organization not found",
				fmt.Sprintf("No organization with name '%s' found", filter.Name.ValueString()),
			)
			return
		}
	}

	// Set the data for the response
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapOrganizationToModel converts an API response to a data model
func (d *OrganizationDataSource) mapOrganizationToModel(ctx context.Context, data *OrganizationDataSourceModel, apiResp OrganizationAPIResponse, resp *datasource.ReadResponse) {
	data.ID = types.StringValue(apiResp.ID)
	data.Name = types.StringValue(apiResp.Name)
	data.CreatedAt = types.StringValue(apiResp.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(apiResp.UpdatedAt.Format(time.RFC3339))
	data.ExecutionMode = types.StringValue(apiResp.ExecutionMode)
	data.AgentsEnabled = types.BoolValue(apiResp.AgentsEnabled)

	// Map members
	members := make([]OrganizationMemberModel, 0, len(apiResp.Members))
	for _, m := range apiResp.Members {
		members = append(members, OrganizationMemberModel{
			Email: types.StringValue(m.Email),
		})
	}
	membersValue, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"email": types.StringType,
		},
	}, members)
	resp.Diagnostics.Append(diags...)
	data.Members = membersValue

	// Map teams
	teams := make([]OrganizationTeamModel, 0, len(apiResp.Teams))
	for _, t := range apiResp.Teams {
		teams = append(teams, OrganizationTeamModel{
			Name: types.StringValue(t.Name),
		})
	}
	teamsValue, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name": types.StringType,
		},
	}, teams)
	resp.Diagnostics.Append(diags...)
	data.Teams = teamsValue
}

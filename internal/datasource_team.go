package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &TeamDataSource{}

func NewTeamDataSource() datasource.DataSource {
	return &TeamDataSource{}
}

type TeamDataSource struct {
	provider *InfradotsProvider
}

type TeamDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	Members          types.List   `tfsdk:"members"`
}

func (d *TeamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_data"
}

func (d *TeamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a team by ID or by organization name and team name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the team.",
				Optional:    true,
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the team.",
				Optional:    true,
				Computed:    true,
			},
			"members": schema.ListAttribute{
				Description: "List of member email addresses in the team.",
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

func (d *TeamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TeamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TeamDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgName := data.OrganizationName.ValueString()

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		// Fetch by ID
		url := fmt.Sprintf("https://%s/api/organizations/%s/teams/%s/",
			d.provider.host, orgName, data.ID.ValueString())

		httpReq, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error creating request", err.Error())
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+d.provider.token)

		httpResp, err := d.provider.client.Do(httpReq)
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != 200 {
			resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
			return
		}

		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			resp.Diagnostics.AddError("Error reading response body", err.Error())
			return
		}

		var team TeamAPIResponse
		err = json.Unmarshal(body, &team)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		data.ID = types.StringValue(team.ID)
		data.Name = types.StringValue(team.Name)
		data.Members = teamMembersToList(team.Members)
	} else if !data.Name.IsNull() && data.Name.ValueString() != "" {
		// List and filter by name
		url := fmt.Sprintf("https://%s/api/organizations/%s/teams/",
			d.provider.host, orgName)

		httpReq, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error creating request", err.Error())
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+d.provider.token)

		httpResp, err := d.provider.client.Do(httpReq)
		if err != nil {
			resp.Diagnostics.AddError("HTTP request failed", err.Error())
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != 200 {
			resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
			return
		}

		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			resp.Diagnostics.AddError("Error reading response body", err.Error())
			return
		}

		var teams []TeamAPIResponse
		err = json.Unmarshal(body, &teams)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		found := false
		for _, team := range teams {
			if team.Name == data.Name.ValueString() {
				data.ID = types.StringValue(team.ID)
				data.Name = types.StringValue(team.Name)
				data.Members = teamMembersToList(team.Members)
				found = true
				break
			}
		}

		if !found {
			resp.Diagnostics.AddError(
				"Team not found",
				fmt.Sprintf("No team with name '%s' found in organization '%s'", data.Name.ValueString(), orgName),
			)
			return
		}
	} else {
		resp.Diagnostics.AddError("Missing required parameter", "Either id or name must be specified")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

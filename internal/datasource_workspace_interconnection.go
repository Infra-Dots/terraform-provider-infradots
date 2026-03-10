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

var _ datasource.DataSource = &WorkspaceInterconnectionDataSource{}

func NewWorkspaceInterconnectionDataSource() datasource.DataSource {
	return &WorkspaceInterconnectionDataSource{}
}

type WorkspaceInterconnectionDataSource struct {
	provider *InfradotsProvider
}

type WorkspaceInterconnectionDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
	ToWorkspaces     types.List   `tfsdk:"to_workspaces"`
	Condition        types.String `tfsdk:"condition"`
}

type InterconnectionDataSourceAPIResponse struct {
	ID                  interface{}               `json:"id"`
	Condition           string                    `json:"condition"`
	ConnectedWorkspaces []InterconnectedWorkspace `json:"connected_workspaces"`
}

func (d *WorkspaceInterconnectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_interconnection_data"
}

func (d *WorkspaceInterconnectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches workspace interconnection details for a given workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the interconnection.",
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization.",
				Required:    true,
			},
			"workspace_name": schema.StringAttribute{
				Description: "The name of the workspace.",
				Required:    true,
			},
			"to_workspaces": schema.ListAttribute{
				Description: "List of workspace names connected to this workspace.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"condition": schema.StringAttribute{
				Description: "The condition for triggering connected workspaces.",
				Computed:    true,
			},
		},
	}
}

func (d *WorkspaceInterconnectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkspaceInterconnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceInterconnectionDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiURL := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/connect_workspaces/",
		d.provider.host,
		data.OrganizationName.ValueString(),
		data.WorkspaceName.ValueString())

	httpReq, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.provider.token)

	httpResp, err := d.provider.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error making HTTP request", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Unexpected HTTP status code",
			fmt.Sprintf("Expected 200, got: %d", httpResp.StatusCode),
		)
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var apiResp InterconnectionDataSourceAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Build the id string from the composite key
	data.ID = types.StringValue(data.OrganizationName.ValueString() + ":" + data.WorkspaceName.ValueString())
	data.Condition = types.StringValue(apiResp.Condition)

	// Collect workspace names from connected_workspaces
	toWorkspaces := make([]string, 0, len(apiResp.ConnectedWorkspaces))
	for _, ws := range apiResp.ConnectedWorkspaces {
		toWorkspaces = append(toWorkspaces, ws.Name)
	}

	listVal, diags := types.ListValueFrom(ctx, types.StringType, toWorkspaces)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ToWorkspaces = listVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

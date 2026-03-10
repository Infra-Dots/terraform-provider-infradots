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

var _ datasource.DataSource = &WorkspaceScheduleDataSource{}

func NewWorkspaceScheduleDataSource() datasource.DataSource {
	return &WorkspaceScheduleDataSource{}
}

type WorkspaceScheduleDataSource struct {
	provider *InfradotsProvider
}

type WorkspaceScheduleDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
	Type             types.String `tfsdk:"type"`
	Crontab          types.String `tfsdk:"crontab"`
	Schedule         types.String `tfsdk:"schedule"`
}

func (d *WorkspaceScheduleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_schedule_data"
}

func (d *WorkspaceScheduleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a workspace schedule by ID or by type.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the workspace schedule.",
				Optional:    true,
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization the workspace belongs to.",
				Required:    true,
			},
			"workspace_name": schema.StringAttribute{
				Description: "The name of the workspace.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The type of schedule (e.g., plan, apply). Used as a filter when id is not provided.",
				Optional:    true,
				Computed:    true,
			},
			"crontab": schema.StringAttribute{
				Description: "The crontab expression for the schedule.",
				Computed:    true,
			},
			"schedule": schema.StringAttribute{
				Description: "A human-readable description of the schedule.",
				Computed:    true,
			},
		},
	}
}

func (d *WorkspaceScheduleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkspaceScheduleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceScheduleDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgName := data.OrganizationName.ValueString()
	wsName := data.WorkspaceName.ValueString()

	if data.ID.IsNull() && data.Type.IsNull() {
		resp.Diagnostics.AddError(
			"Missing required parameter",
			"Either id or type must be provided",
		)
		return
	}

	if !data.ID.IsNull() {
		// Fetch single schedule by ID
		url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/schedules/%s/",
			d.provider.host, orgName, wsName, data.ID.ValueString())

		httpReq, err := http.NewRequest(http.MethodGet, url, nil)
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

		var apiResp WorkspaceScheduleAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		data.ID = types.StringValue(apiResp.ID)
		data.Type = types.StringValue(apiResp.Type)
		data.Crontab = types.StringValue(apiResp.Crontab)
		data.Schedule = types.StringValue(apiResp.Schedule)
	} else {
		// Fetch list and filter by type
		url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/schedules/",
			d.provider.host, orgName, wsName)

		httpReq, err := http.NewRequest(http.MethodGet, url, nil)
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

		var apiRespList []WorkspaceScheduleAPIResponse
		if err := json.Unmarshal(body, &apiRespList); err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		filterType := data.Type.ValueString()
		found := false
		for _, s := range apiRespList {
			if s.Type == filterType {
				data.ID = types.StringValue(s.ID)
				data.Type = types.StringValue(s.Type)
				data.Crontab = types.StringValue(s.Crontab)
				data.Schedule = types.StringValue(s.Schedule)
				found = true
				break
			}
		}

		if !found {
			resp.Diagnostics.AddError(
				"Workspace schedule not found",
				fmt.Sprintf("No schedule with type '%s' found for workspace '%s' in organization '%s'",
					filterType, wsName, orgName),
			)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

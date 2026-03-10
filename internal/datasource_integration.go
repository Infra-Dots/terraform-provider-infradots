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
var _ datasource.DataSource = &IntegrationDataSource{}

func NewIntegrationDataSource() datasource.DataSource {
	return &IntegrationDataSource{}
}

type IntegrationDataSource struct {
	provider *InfradotsProvider
}

type IntegrationDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	APIURL           types.String `tfsdk:"api_url"`
	Description      types.String `tfsdk:"description"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

type IntegrationDataSourceFilterModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	APIURL           types.String `tfsdk:"api_url"`
	Description      types.String `tfsdk:"description"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func (d *IntegrationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_data"
}

func (d *IntegrationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an integration by ID (with organization_name) or by organization_name and name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the integration.",
				Optional:    true,
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this integration belongs to.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the integration.",
				Optional:    true,
				Computed:    true,
			},
			"type": schema.StringAttribute{
				Description: "The type of integration (e.g., WEBHOOK, CUSTOM, SLACK).",
				Computed:    true,
			},
			"api_url": schema.StringAttribute{
				Description: "The API URL for the integration.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the integration.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the integration was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the integration was last updated.",
				Computed:    true,
			},
		},
	}
}

func (d *IntegrationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IntegrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var filter IntegrationDataSourceFilterModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &filter)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if filter.OrganizationName.IsNull() || filter.OrganizationName.IsUnknown() {
		resp.Diagnostics.AddError(
			"Missing required parameter",
			"organization_name must be specified",
		)
		return
	}

	org := filter.OrganizationName.ValueString()

	var data IntegrationDataSourceModel

	if !filter.ID.IsNull() && !filter.ID.IsUnknown() {
		// Fetch by ID
		url := fmt.Sprintf("https://%s/api/organizations/%s/integrations/%s/",
			d.provider.host, org, filter.ID.ValueString())

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

		var apiResp IntegrationAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		data.ID = types.StringValue(apiResp.ID)
		data.OrganizationName = types.StringValue(org)
		data.Name = types.StringValue(apiResp.Name)
		data.Type = types.StringValue(apiResp.Type)
		data.APIURL = types.StringValue(apiResp.APIURL)
		data.Description = types.StringValue(apiResp.Description)
		data.CreatedAt = types.StringValue(apiResp.CreatedAt.Format(time.RFC3339))
		data.UpdatedAt = types.StringValue(apiResp.UpdatedAt.Format(time.RFC3339))
	} else if !filter.Name.IsNull() && !filter.Name.IsUnknown() {
		// Fetch by name: list and filter
		url := fmt.Sprintf("https://%s/api/organizations/%s/integrations/",
			d.provider.host, org)

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

		var apiRespList []IntegrationAPIResponse
		if err := json.Unmarshal(body, &apiRespList); err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		found := false
		for _, apiResp := range apiRespList {
			if apiResp.Name == filter.Name.ValueString() {
				data.ID = types.StringValue(apiResp.ID)
				data.OrganizationName = types.StringValue(org)
				data.Name = types.StringValue(apiResp.Name)
				data.Type = types.StringValue(apiResp.Type)
				data.APIURL = types.StringValue(apiResp.APIURL)
				data.Description = types.StringValue(apiResp.Description)
				data.CreatedAt = types.StringValue(apiResp.CreatedAt.Format(time.RFC3339))
				data.UpdatedAt = types.StringValue(apiResp.UpdatedAt.Format(time.RFC3339))
				found = true
				break
			}
		}

		if !found {
			resp.Diagnostics.AddError(
				"Integration not found",
				fmt.Sprintf("No integration with name '%s' found in organization '%s'",
					filter.Name.ValueString(), org),
			)
			return
		}
	} else {
		resp.Diagnostics.AddError(
			"Missing required parameter",
			"Either id or name must be specified alongside organization_name",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

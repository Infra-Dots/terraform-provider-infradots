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

var _ datasource.DataSource = &ModelProviderDataSource{}

func NewModelProviderDataSource() datasource.DataSource {
	return &ModelProviderDataSource{}
}

type ModelProviderDataSource struct {
	provider *InfradotsProvider
}

type ModelProviderDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	Provider         types.String `tfsdk:"provider"`
	Description      types.String `tfsdk:"description"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func (d *ModelProviderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_provider_data"
}

func (d *ModelProviderDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a model provider by ID or by organization name and provider name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the model provider.",
				Optional:    true,
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the model provider.",
				Optional:    true,
				Computed:    true,
			},
			"provider": schema.StringAttribute{
				Description: "The provider type (e.g., openai, anthropic, google, azure_openai, cohere, llama).",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the model provider.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the model provider was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the model provider was last updated.",
				Computed:    true,
			},
		},
	}
}

func (d *ModelProviderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ModelProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ModelProviderDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.OrganizationName.IsNull() {
		resp.Diagnostics.AddError(
			"Missing required parameter",
			"organization_name must be specified",
		)
		return
	}

	if data.ID.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing required parameter",
			"Either id or name must be specified",
		)
		return
	}

	orgName := data.OrganizationName.ValueString()

	if !data.ID.IsNull() {
		// Fetch single by ID
		url := fmt.Sprintf("https://%s/api/organizations/%s/model-providers/%s/",
			d.provider.host,
			orgName,
			data.ID.ValueString())

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
			resp.Diagnostics.AddError(
				"Unexpected HTTP status",
				fmt.Sprintf("Expected 200, got: %d", httpResp.StatusCode),
			)
			return
		}

		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			resp.Diagnostics.AddError("Error reading response body", err.Error())
			return
		}

		var mp ModelProviderAPIResponse
		err = json.Unmarshal(body, &mp)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		data.ID = types.StringValue(mp.ID)
		data.OrganizationName = types.StringValue(orgName)
		data.Name = types.StringValue(mp.Name)
		data.Provider = types.StringValue(mp.Provider)
		data.Description = types.StringValue(mp.Description)
		data.CreatedAt = types.StringValue(mp.CreatedAt)
		data.UpdatedAt = types.StringValue(mp.UpdatedAt)
	} else {
		// List and filter by name
		url := fmt.Sprintf("https://%s/api/organizations/%s/model-providers/",
			d.provider.host,
			orgName)

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
			resp.Diagnostics.AddError(
				"Unexpected HTTP status",
				fmt.Sprintf("Expected 200, got: %d", httpResp.StatusCode),
			)
			return
		}

		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			resp.Diagnostics.AddError("Error reading response body", err.Error())
			return
		}

		var providers []ModelProviderAPIResponse
		err = json.Unmarshal(body, &providers)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing response", err.Error())
			return
		}

		found := false
		for _, mp := range providers {
			if mp.Name == data.Name.ValueString() {
				data.ID = types.StringValue(mp.ID)
				data.OrganizationName = types.StringValue(orgName)
				data.Name = types.StringValue(mp.Name)
				data.Provider = types.StringValue(mp.Provider)
				data.Description = types.StringValue(mp.Description)
				data.CreatedAt = types.StringValue(mp.CreatedAt)
				data.UpdatedAt = types.StringValue(mp.UpdatedAt)
				found = true
				break
			}
		}

		if !found {
			resp.Diagnostics.AddError(
				"Model provider not found",
				fmt.Sprintf("No model provider with name '%s' found in organization '%s'",
					data.Name.ValueString(), orgName),
			)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

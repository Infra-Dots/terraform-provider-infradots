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

var _ datasource.DataSource = &VariableDataSource{}

func NewVariableDataSource() datasource.DataSource {
	return &VariableDataSource{}
}

type VariableDataSource struct {
	provider *InfradotsProvider
}

type VariableDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
	Key              types.String `tfsdk:"key"`
	Value            types.String `tfsdk:"value"`
	Category         types.String `tfsdk:"category"`
	Sensitive        types.Bool   `tfsdk:"sensitive"`
	HCL              types.Bool   `tfsdk:"hcl"`
	Description      types.String `tfsdk:"description"`
}

func (d *VariableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_variable_data"
}

func (d *VariableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a variable by key from an organization or workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the variable.",
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization.",
				Required:    true,
			},
			"workspace_name": schema.StringAttribute{
				Description: "The name of the workspace. If provided, fetches a workspace variable; if absent, fetches an org variable.",
				Optional:    true,
			},
			"key": schema.StringAttribute{
				Description: "The name/key of the variable to look up.",
				Required:    true,
			},
			"value": schema.StringAttribute{
				Description: "The value of the variable.",
				Computed:    true,
				Sensitive:   true,
			},
			"category": schema.StringAttribute{
				Description: "The category of the variable (e.g., terraform or env).",
				Computed:    true,
			},
			"sensitive": schema.BoolAttribute{
				Description: "Whether the variable contains sensitive data.",
				Computed:    true,
			},
			"hcl": schema.BoolAttribute{
				Description: "Whether the value is parsed as HCL.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the variable.",
				Computed:    true,
			},
		},
	}
}

func (d *VariableDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VariableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VariableDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var apiURL string
	if !data.WorkspaceName.IsNull() && data.WorkspaceName.ValueString() != "" {
		apiURL = fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/variables/",
			d.provider.host,
			data.OrganizationName.ValueString(),
			data.WorkspaceName.ValueString())
	} else {
		apiURL = fmt.Sprintf("https://%s/api/organizations/%s/variables/",
			d.provider.host,
			data.OrganizationName.ValueString())
	}

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

	var variables []VariableAPIResponse
	if err := json.Unmarshal(body, &variables); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	found := false
	for _, v := range variables {
		if v.Key == data.Key.ValueString() {
			data.ID = types.StringValue(v.ID)
			data.Key = types.StringValue(v.Key)
			data.Value = types.StringValue(v.Value)
			data.Category = types.StringValue(v.Category)
			data.Sensitive = types.BoolValue(v.Sensitive)
			data.HCL = types.BoolValue(v.HCL)
			data.Description = types.StringValue(v.Description)
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Variable not found",
			fmt.Sprintf("No variable with key '%s' found", data.Key.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

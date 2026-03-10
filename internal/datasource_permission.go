package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &PermissionDataSource{}

func NewPermissionDataSource() datasource.DataSource {
	return &PermissionDataSource{}
}

type PermissionDataSource struct {
	provider *InfradotsProvider
}

type PermissionDataSourceModel struct {
	OrganizationName types.String `tfsdk:"organization_name"`
	PermissionName   types.String `tfsdk:"permission_name"`
	TeamID           types.String `tfsdk:"team_id"`
	UserEmail        types.String `tfsdk:"user_email"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
	Permissions      types.List   `tfsdk:"permissions"`
}

type PermissionDataSourceAPIResponse struct {
	ID             string `json:"id"`
	PermissionName string `json:"permission_name"`
	TeamID         string `json:"team_id"`
	UserEmail      string `json:"user_email"`
	WorkspaceName  string `json:"workspace_name"`
}

func (d *PermissionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_permission_data"
}

func (d *PermissionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches permission mappings for an organization with optional filters.",
		Attributes: map[string]schema.Attribute{
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization.",
				Required:    true,
			},
			"permission_name": schema.StringAttribute{
				Description: "Filter by permission name.",
				Optional:    true,
			},
			"team_id": schema.StringAttribute{
				Description: "Filter by team ID.",
				Optional:    true,
			},
			"user_email": schema.StringAttribute{
				Description: "Filter by user email.",
				Optional:    true,
			},
			"workspace_name": schema.StringAttribute{
				Description: "Filter by workspace name.",
				Optional:    true,
			},
			"permissions": schema.ListNestedAttribute{
				Description: "List of matching permission mappings.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"permission_name": schema.StringAttribute{
							Computed: true,
						},
						"team_id": schema.StringAttribute{
							Computed: true,
						},
						"user_email": schema.StringAttribute{
							Computed: true,
						},
						"workspace_name": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (d *PermissionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PermissionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PermissionDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawURL := fmt.Sprintf("https://%s/api/permissions/%s/",
		d.provider.host,
		data.OrganizationName.ValueString())

	u, err := url.Parse(rawURL)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing URL", err.Error())
		return
	}

	q := u.Query()
	if !data.PermissionName.IsNull() && data.PermissionName.ValueString() != "" {
		q.Set("permission_name", data.PermissionName.ValueString())
	}
	if !data.TeamID.IsNull() && data.TeamID.ValueString() != "" {
		q.Set("team_id", data.TeamID.ValueString())
	}
	if !data.UserEmail.IsNull() && data.UserEmail.ValueString() != "" {
		q.Set("user_email", data.UserEmail.ValueString())
	}
	if !data.WorkspaceName.IsNull() && data.WorkspaceName.ValueString() != "" {
		q.Set("workspace_name", data.WorkspaceName.ValueString())
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequest(http.MethodGet, u.String(), nil)
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

	var permissions []PermissionDataSourceAPIResponse
	if err := json.Unmarshal(body, &permissions); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	permType := map[string]attr.Type{
		"id":              types.StringType,
		"permission_name": types.StringType,
		"team_id":         types.StringType,
		"user_email":      types.StringType,
		"workspace_name":  types.StringType,
	}

	var permVals []attr.Value
	for _, p := range permissions {
		obj, diags := types.ObjectValue(permType, map[string]attr.Value{
			"id":              types.StringValue(p.ID),
			"permission_name": types.StringValue(p.PermissionName),
			"team_id":         types.StringValue(p.TeamID),
			"user_email":      types.StringValue(p.UserEmail),
			"workspace_name":  types.StringValue(p.WorkspaceName),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		permVals = append(permVals, obj)
	}

	if permVals == nil {
		permVals = []attr.Value{}
	}

	listVal, diags := types.ListValue(types.ObjectType{AttrTypes: permType}, permVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = listVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

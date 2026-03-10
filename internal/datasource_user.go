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

var _ datasource.DataSource = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	provider *InfradotsProvider
}

type UserDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Email            types.String `tfsdk:"email"`
	Name             types.String `tfsdk:"name"`
}

type UserListAPIResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_data"
}

func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a user by email address from an organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique ID of the user.",
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization.",
				Required:    true,
			},
			"email": schema.StringAttribute{
				Description: "The email address of the user to look up.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the user.",
				Computed:    true,
			},
		},
	}
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiURL := fmt.Sprintf("https://%s/api/users/%s/users/",
		d.provider.host,
		data.OrganizationName.ValueString())

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

	var users []UserListAPIResponse
	if err := json.Unmarshal(body, &users); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	found := false
	for _, u := range users {
		if u.Email == data.Email.ValueString() {
			data.ID = types.StringValue(u.ID)
			data.Email = types.StringValue(u.Email)
			data.Name = types.StringValue(u.Name)
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"User not found",
			fmt.Sprintf("No user with email '%s' found in organization '%s'",
				data.Email.ValueString(),
				data.OrganizationName.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

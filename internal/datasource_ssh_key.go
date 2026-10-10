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

var _ datasource.DataSource = &SshKeyDataSource{}

func NewSshKeyDataSource() datasource.DataSource {
	return &SshKeyDataSource{}
}

// SshKeyDataSource looks an organization's SSH key up by name, e.g. one added in the web app, to set
// `ssh_key_id` on a workspace. Never the private key.
type SshKeyDataSource struct {
	provider *InfradotsProvider
}

type SshKeyDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	PublicKey        types.String `tfsdk:"public_key"`
	Fingerprint      types.String `tfsdk:"fingerprint"`
	KnownHosts       types.String `tfsdk:"known_hosts"`
}

func (d *SshKeyDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "infradots_ssh_key"
}

func (d *SshKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An organization's SSH key, by name. The private key is never returned.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Description: "The SSH key's ID.", Computed: true},
			"organization_name": schema.StringAttribute{Description: "The organization.", Required: true},
			"name":              schema.StringAttribute{Description: "The key's name.", Required: true},
			"public_key":        schema.StringAttribute{Description: "The key's public half (OpenSSH format).", Computed: true},
			"fingerprint":       schema.StringAttribute{Description: "The key's SHA256 fingerprint.", Computed: true},
			"known_hosts":       schema.StringAttribute{Description: "The key's known_hosts lines.", Computed: true},
		},
	}
}

func (d *SshKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			d.provider = provider
		}
	}
}

func (d *SshKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SshKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	url := fmt.Sprintf("https://%s/api/organizations/%s/ssh-keys/", d.provider.host, data.OrganizationName.ValueString())
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
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Error listing SSH keys", fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(body)))
		return
	}
	var keys []SshKeyAPIResponse
	if err := json.Unmarshal(body, &keys); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}
	for _, key := range keys {
		if key.Name == data.Name.ValueString() {
			data.ID = types.StringValue(key.ID)
			data.PublicKey = types.StringValue(key.PublicKey)
			data.Fingerprint = types.StringValue(key.Fingerprint)
			data.KnownHosts = types.StringValue(key.KnownHosts)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.Diagnostics.AddError("SSH key not found",
		fmt.Sprintf("No SSH key named %q in organization %s.", data.Name.ValueString(), data.OrganizationName.ValueString()))
}

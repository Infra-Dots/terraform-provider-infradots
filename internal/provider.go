package internal

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ provider.Provider = &InfradotsProvider{}

type InfradotsProviderModel struct {
	Hostname              types.String `tfsdk:"hostname"`
	Token                 types.String `tfsdk:"token"`
	TLSInsecureSkipVerify types.Bool   `tfsdk:"tls_insecure_skip_verify"`
}

type InfradotsProvider struct {
	client *http.Client
	host   string
	token  string
}

func NewProvider() provider.Provider {
	return &InfradotsProvider{}
}

// Metadata returns the provider type name.
func (p *InfradotsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "infradots"
}

// Schema defines the provider-level configuration schema.
func (p *InfradotsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"hostname": schema.StringAttribute{
				Description: "The hostname of the Infradots Platform.",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "API token for authenticating requests.",
				Required:    true,
				Sensitive:   true,
			},
			"tls_insecure_skip_verify": schema.BoolAttribute{
				Description: "If true, skips TLS certificate verification (not recommended for production).",
				Optional:    true,
			},
		},
	}
}

func (p *InfradotsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config InfradotsProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set default value for tls_insecure_skip_verify if not provided
	tlsInsecureSkipVerify := true
	if !config.TLSInsecureSkipVerify.IsNull() {
		tlsInsecureSkipVerify = config.TLSInsecureSkipVerify.ValueBool()
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: tlsInsecureSkipVerify,
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	httpClient := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	p.client = httpClient
	if config.Hostname.IsNull() {
		p.host = "api.infradots.com"
	} else {
		p.host = config.Hostname.ValueString()
	}

	p.token = config.Token.ValueString()
	tflog.Info(ctx, "Creating infradots client information", map[string]any{"success": true})

	resp.ResourceData = p
	resp.DataSourceData = p
}

// Resources returns the list of resource implementations.
func (p *InfradotsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewOrganizationResource,
		NewWorkspaceResource,
		NewVariableResource,
		NewVCSResource,
	}
}

// DataSources returns the list of data source implementations.
func (p *InfradotsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
		NewWorkspaceDataSource,
		NewVCSDataSource,
	}
}

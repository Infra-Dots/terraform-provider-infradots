package internal

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestTeamDataSource(t *testing.T) *TeamDataSource {
	t.Helper()
	httpClient := &http.Client{Transport: &MockTeamRoundTripper{}}
	provider := &InfradotsProvider{
		host:   "api.infradots.com",
		token:  "test-token",
		client: httpClient,
	}
	return &TeamDataSource{provider: provider}
}

func TestTeamDataSource_Metadata(t *testing.T) {
	d := NewTeamDataSource()
	ctx := context.Background()
	resp := &datasource.MetadataResponse{}
	req := datasource.MetadataRequest{ProviderTypeName: "infradots"}
	d.Metadata(ctx, req, resp)
	assert.Equal(t, "infradots_team_data", resp.TypeName)
}

func TestTeamDataSource_Schema(t *testing.T) {
	d := NewTeamDataSource()
	ctx := context.Background()
	resp := &datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, resp)

	attrs := resp.Schema.Attributes
	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "name")
	assert.Contains(t, attrs, "members")

	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Optional)
	assert.True(t, idAttr.Computed)

	orgAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgAttr.Required)

	nameAttr := attrs["name"].(schema.StringAttribute)
	assert.True(t, nameAttr.Optional)
	assert.True(t, nameAttr.Computed)
}

func TestTeamDataSource_Configure(t *testing.T) {
	d := setupTestTeamDataSource(t)
	ctx := context.Background()

	provider := &InfradotsProvider{
		host:   "api.infradots.com",
		token:  "test-token",
		client: &http.Client{},
	}

	req := datasource.ConfigureRequest{ProviderData: provider}
	resp := &datasource.ConfigureResponse{}
	d.Configure(ctx, req, resp)

	require.False(t, resp.Diagnostics.HasError())
	assert.NotNil(t, d.provider)
}

func TestTeamDataSource_ConfigureInvalidProvider(t *testing.T) {
	d := setupTestTeamDataSource(t)
	ctx := context.Background()

	req := datasource.ConfigureRequest{ProviderData: "invalid"}
	resp := &datasource.ConfigureResponse{}
	d.Configure(ctx, req, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Summary(), "Unexpected Data Source Configure Type")
}

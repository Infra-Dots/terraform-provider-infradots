package internal

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockVCSDataSourceRoundTripper implements http.RoundTripper for testing VCS data source
type MockVCSDataSourceRoundTripper struct{}

func (m *MockVCSDataSourceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a mocked response based on the request
	resp := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	resp.Header.Set("Content-Type", "application/json")

	// Check the URL and method to determine response
	url := req.URL.String()

	// Handle single VCS by ID (GET to /api/organizations/{org_name}/vcs/{id})
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/vcs/5f560f5e-0bf3-6543-defg-g1156789012c") {
		jsonResp := `{
			"id": "5f560f5e-0bf3-6543-defg-g1156789012c",
			"name": "test-vcs",
			"vcs_type": "github",
			"url": "https://github.com",
			"clientId": "test-client-id",
			"description": "Test VCS connection for GitHub",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Handle list of VCS connections (GET to /api/organizations/{org_name}/vcs/)
	if req.Method == http.MethodGet && strings.Contains(url, "/api/organizations/test-org/vcs/") && !strings.Contains(url, "/5f560f5e-0bf3-6543-defg-g1156789012c") {
		jsonResp := `[{
			"id": "5f560f5e-0bf3-6543-defg-g1156789012c",
			"name": "test-vcs",
			"vcs_type": "github",
			"url": "https://github.com",
			"clientId": "test-client-id",
			"description": "Test VCS connection for GitHub",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}, {
			"id": "6f670f6f-1cf4-7654-efgh-h2267890123d",
			"name": "gitlab-vcs",
			"vcs_type": "gitlab",
			"url": "https://gitlab.com",
			"clientId": "gitlab-client-id",
			"description": "GitLab VCS connection",
			"created_at": "2025-07-07T12:00:00Z",
			"updated_at": "2025-07-07T12:00:00Z"
		}]`
		resp.Body = io.NopCloser(strings.NewReader(jsonResp))
		return resp, nil
	}

	// Default: return a 404 Not Found
	resp.StatusCode = http.StatusNotFound
	resp.Body = io.NopCloser(strings.NewReader(`{"error": "Not found"}`))
	return resp, nil
}

// setupTestVCSDataSource sets up a test data source with a mock client
func setupTestVCSDataSource(t *testing.T) *VCSDataSource {
	t.Helper()

	// Create a client with a mock transport
	httpClient := &http.Client{
		Transport: &MockVCSDataSourceRoundTripper{},
	}

	// Create a provider with the mock client
	provider := &InfradotsProvider{
		host:   "api.infradots.com", // This value is not actually used in tests
		token:  "test-token",
		client: httpClient,
	}

	dataSource := &VCSDataSource{
		provider: provider,
	}

	return dataSource
}

func TestVCSDataSource_Schema(t *testing.T) {
	d := NewVCSDataSource()

	ctx := context.Background()
	resp := &datasource.SchemaResponse{}

	d.Schema(ctx, datasource.SchemaRequest{}, resp)

	// Verify the schema attributes
	attrs := resp.Schema.Attributes

	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "organization_name")
	assert.Contains(t, attrs, "name")
	assert.Contains(t, attrs, "vcs_type")
	assert.Contains(t, attrs, "url")
	assert.Contains(t, attrs, "clientId")
	assert.Contains(t, attrs, "description")
	assert.Contains(t, attrs, "created_at")
	assert.Contains(t, attrs, "updated_at")

	// Check specific attribute properties
	idAttr := attrs["id"].(schema.StringAttribute)
	assert.True(t, idAttr.Optional)
	assert.True(t, idAttr.Computed)

	orgNameAttr := attrs["organization_name"].(schema.StringAttribute)
	assert.True(t, orgNameAttr.Optional)
	assert.True(t, orgNameAttr.Computed)

	nameAttr := attrs["name"].(schema.StringAttribute)
	assert.True(t, nameAttr.Optional)
	assert.True(t, nameAttr.Computed)

	vcsTypeAttr := attrs["vcs_type"].(schema.StringAttribute)
	assert.True(t, vcsTypeAttr.Computed)

	urlAttr := attrs["url"].(schema.StringAttribute)
	assert.True(t, urlAttr.Computed)

	clientIdAttr := attrs["clientId"].(schema.StringAttribute)
	assert.True(t, clientIdAttr.Computed)

	descAttr := attrs["description"].(schema.StringAttribute)
	assert.True(t, descAttr.Computed)

	createdAtAttr := attrs["created_at"].(schema.StringAttribute)
	assert.True(t, createdAtAttr.Computed)

	updatedAtAttr := attrs["updated_at"].(schema.StringAttribute)
	assert.True(t, updatedAtAttr.Computed)
}

func TestVCSDataSource_Metadata(t *testing.T) {
	d := NewVCSDataSource()

	ctx := context.Background()
	resp := &datasource.MetadataResponse{}

	req := datasource.MetadataRequest{
		ProviderTypeName: "infradots",
	}
	d.Metadata(ctx, req, resp)

	// Verify the type name
	assert.Equal(t, "infradots_vcs_data", resp.TypeName)
}

func TestVCSDataSource_Configure(t *testing.T) {
	d := setupTestVCSDataSource(t)

	ctx := context.Background()

	// Test with valid provider data
	provider := &InfradotsProvider{
		host:   "api.infradots.com",
		token:  "test-token",
		client: &http.Client{},
	}

	req := datasource.ConfigureRequest{
		ProviderData: provider,
	}
	resp := &datasource.ConfigureResponse{}

	d.Configure(ctx, req, resp)

	// Should not have errors
	require.False(t, resp.Diagnostics.HasError())
	assert.NotNil(t, d.provider)
}

func TestVCSDataSource_ConfigureInvalidProvider(t *testing.T) {
	d := setupTestVCSDataSource(t)

	ctx := context.Background()

	// Test with invalid provider data
	req := datasource.ConfigureRequest{
		ProviderData: "invalid",
	}
	resp := &datasource.ConfigureResponse{}

	d.Configure(ctx, req, resp)

	// Should have errors
	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Summary(), "Unexpected Data Source Configure Type")
}

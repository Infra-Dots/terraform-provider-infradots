package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SshKeyResource{}
var _ resource.ResourceWithImportState = &SshKeyResource{}

func NewSshKeyResource() resource.Resource {
	return &SshKeyResource{}
}

// SshKeyResource is an organization's SSH key for module sources fetched over SSH. The private key is
// write-only: the API returns only its public half and fingerprint.
type SshKeyResource struct {
	provider *InfradotsProvider
}

type SshKeyResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	PrivateKey       types.String `tfsdk:"private_key"`
	KnownHosts       types.String `tfsdk:"known_hosts"`
	PublicKey        types.String `tfsdk:"public_key"`
	Fingerprint      types.String `tfsdk:"fingerprint"`
}

type SshKeyAPIResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	PublicKey   string   `json:"public_key"`
	Fingerprint string   `json:"fingerprint"`
	KnownHosts  string   `json:"known_hosts"`
	Workspaces  []string `json:"workspaces"`
}

type SshKeyRequest struct {
	Name       string  `json:"name,omitempty"`
	PrivateKey string  `json:"private_key,omitempty"`
	KnownHosts *string `json:"known_hosts,omitempty"`
}

func (r *SshKeyResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_ssh_key"
}

func (r *SshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An organization's SSH key for module sources fetched over SSH (`git::ssh://...`, " +
			"`git@github.com:...`). A workspace uses it with `ssh_key_id`; each of its jobs hands it to the executor " +
			"for the run. The private key is write-only: it is encrypted in InfraDots and never read back.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The SSH key's ID.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_name": schema.StringAttribute{
				Description:   "The organization the key belongs to.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "The key's name, unique in the organization.",
				Required:    true,
			},
			"private_key": schema.StringAttribute{
				Description: "The private key, OpenSSH or PEM, without a passphrase. Write-only: never read back.",
				Required:    true,
				Sensitive:   true,
			},
			"known_hosts": schema.StringAttribute{
				Description: "known_hosts lines for SSH hosts other than github.com, gitlab.com and bitbucket.org, " +
					"whose host keys the executors already trust.",
				Optional: true,
				Computed: true,
			},
			"public_key": schema.StringAttribute{
				Description:   "The key's public half (OpenSSH format): add it as a deploy key to the module repositories.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"fingerprint": schema.StringAttribute{
				Description:   "The key's SHA256 fingerprint, as `ssh-keygen -lf` prints it.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *SshKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *SshKeyResource) url(org, id string) string {
	base := fmt.Sprintf("https://%s/api/organizations/%s/ssh-keys/", r.provider.host, org)
	if id == "" {
		return base
	}
	return base + id + "/"
}

// call sends a request and decodes the SSH key in the response; found is false on a 404.
func (r *SshKeyResource) call(method, url string, body any, okStatus int) (key SshKeyAPIResponse, found bool, err error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return key, false, err
		}
		reader = strings.NewReader(string(payload))
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return key, false, err
	}
	req.Header.Set("Authorization", "Bearer "+r.provider.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.provider.client.Do(req)
	if err != nil {
		return key, false, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return key, false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return key, false, nil
	}
	if resp.StatusCode != okStatus {
		return key, false, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	if method == http.MethodDelete {
		return key, true, nil
	}
	return key, true, json.Unmarshal(respBody, &key)
}

// normalizeKnownHosts is what the API stores: LF line endings, one trailing newline (or nothing).
func normalizeKnownHosts(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if s == "" {
		return ""
	}
	return s + "\n"
}

// apply copies what the API returned into the model. private_key is write-only and left as configured;
// known_hosts keeps the configured spelling while the API's normalised value means the same.
func (m *SshKeyResourceModel) apply(key SshKeyAPIResponse) {
	m.ID = types.StringValue(key.ID)
	m.Name = types.StringValue(key.Name)
	m.PublicKey = types.StringValue(key.PublicKey)
	m.Fingerprint = types.StringValue(key.Fingerprint)
	if m.KnownHosts.IsNull() || m.KnownHosts.IsUnknown() ||
		normalizeKnownHosts(m.KnownHosts.ValueString()) != normalizeKnownHosts(key.KnownHosts) {
		m.KnownHosts = types.StringValue(key.KnownHosts)
	}
}

func (r *SshKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SshKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := SshKeyRequest{Name: data.Name.ValueString(), PrivateKey: data.PrivateKey.ValueString()}
	if !data.KnownHosts.IsNull() && !data.KnownHosts.IsUnknown() {
		knownHosts := data.KnownHosts.ValueString()
		body.KnownHosts = &knownHosts
	}
	key, _, err := r.call(http.MethodPost, r.url(data.OrganizationName.ValueString(), ""), body, http.StatusCreated)
	if err != nil {
		resp.Diagnostics.AddError("Error creating SSH key", err.Error())
		return
	}
	data.apply(key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SshKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SshKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key, found, err := r.call(http.MethodGet, r.url(data.OrganizationName.ValueString(), data.ID.ValueString()), nil, http.StatusOK)
	if err != nil {
		resp.Diagnostics.AddError("Error reading SSH key", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	data.apply(key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SshKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SshKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := SshKeyRequest{}
	if !plan.Name.Equal(state.Name) {
		body.Name = plan.Name.ValueString()
	}
	if !plan.PrivateKey.Equal(state.PrivateKey) {
		body.PrivateKey = plan.PrivateKey.ValueString() // rotates the key
	}
	if !plan.KnownHosts.IsUnknown() && !plan.KnownHosts.Equal(state.KnownHosts) {
		knownHosts := plan.KnownHosts.ValueString()
		body.KnownHosts = &knownHosts
	}
	key, found, err := r.call(http.MethodPatch, r.url(state.OrganizationName.ValueString(), state.ID.ValueString()), body, http.StatusOK)
	if err == nil && !found {
		err = fmt.Errorf("SSH key %s no longer exists", state.ID.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating SSH key", err.Error())
		return
	}
	plan.apply(key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SshKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SshKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, _, err := r.call(http.MethodDelete, r.url(data.OrganizationName.ValueString(), data.ID.ValueString()), nil, http.StatusNoContent)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting SSH key", err.Error())
	}
}

// ImportState takes `organization_name:id`. The private key can't be imported -- set it in the
// configuration; the first apply after the import then rotates the key to that value.
func (r *SshKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Use the format 'organization_name:id'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

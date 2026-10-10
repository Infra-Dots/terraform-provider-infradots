package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sshKeyJSON = `{"id": "key-1", "name": "modules", "public_key": "ssh-ed25519 AAAAC3Nz test",
	"fingerprint": "SHA256:abc", "known_hosts": "git.acme ssh-ed25519 AAAA\n", "workspaces": []}`

const testPrivateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"

func sshKeySchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	resp := resource.SchemaResponse{}
	NewSshKeyResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp
}

func TestSshKeyResource_CreateSendsTheKeyAndNeverReadsItBack(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) { return http.StatusCreated, sshKeyJSON }}
	r := &SshKeyResource{provider: testProvider(transport)}
	ctx := context.Background()
	s := sshKeySchema(t)
	plan := SshKeyResourceModel{OrganizationName: types.StringValue("acme"), Name: types.StringValue("modules"),
		PrivateKey: types.StringValue(testPrivateKey), KnownHosts: types.StringValue("git.acme ssh-ed25519 AAAA"),
		ID: types.StringUnknown(), PublicKey: types.StringUnknown(), Fingerprint: types.StringUnknown()}
	req := resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema}}
	require.Empty(t, req.Plan.Set(ctx, &plan))
	resp := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	sent := transport.requests[0]
	assert.Equal(t, "/api/organizations/acme/ssh-keys/", sent.Path)
	assert.Equal(t, testPrivateKey, sent.Body["private_key"])
	var state SshKeyResourceModel
	require.Empty(t, resp.State.Get(ctx, &state))
	assert.Equal(t, "key-1", state.ID.ValueString())
	assert.Equal(t, "SHA256:abc", state.Fingerprint.ValueString())
	assert.Equal(t, testPrivateKey, state.PrivateKey.ValueString()) // as configured, never from the API
	// The API added a trailing newline; the configured spelling stays, so there's no diff next plan.
	assert.Equal(t, "git.acme ssh-ed25519 AAAA", state.KnownHosts.ValueString())
}

func TestSshKeyModel_KnownHostsFollowsARealChange(t *testing.T) {
	m := SshKeyResourceModel{KnownHosts: types.StringValue("old ssh-ed25519 AAAA")}
	m.apply(SshKeyAPIResponse{ID: "key-1", KnownHosts: "changed.elsewhere ssh-ed25519 BBBB\n"})
	assert.Equal(t, "changed.elsewhere ssh-ed25519 BBBB\n", m.KnownHosts.ValueString())
}

func TestSshKeyResource_ReadForgetsADeletedKey(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) { return http.StatusNotFound, `{}` }}
	r := &SshKeyResource{provider: testProvider(transport)}
	ctx := context.Background()
	s := sshKeySchema(t)
	state := tfsdk.State{Schema: s.Schema}
	require.Empty(t, state.Set(ctx, &SshKeyResourceModel{ID: types.StringValue("key-1"),
		OrganizationName: types.StringValue("acme"), Name: types.StringValue("modules"),
		PrivateKey: types.StringValue(testPrivateKey), KnownHosts: types.StringValue(""),
		PublicKey: types.StringValue(""), Fingerprint: types.StringValue("")}))
	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull())
}

func TestSshKeyRequest_OnlyChangedFieldsAreSent(t *testing.T) {
	encode := func(req SshKeyRequest) map[string]any {
		raw, err := json.Marshal(req)
		require.NoError(t, err)
		var out map[string]any
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}
	assert.Empty(t, encode(SshKeyRequest{}))
	empty := ""
	cleared := encode(SshKeyRequest{KnownHosts: &empty})
	assert.Contains(t, cleared, "known_hosts") // clearing known_hosts sends ""
	assert.NotContains(t, cleared, "private_key")
}

func TestSshKeyDataSource_FindsAKeyByName(t *testing.T) {
	transport := &recordingTransport{respond: func(*http.Request) (int, string) { return http.StatusOK, "[" + sshKeyJSON + "]" }}
	d := &SshKeyDataSource{provider: testProvider(transport)}
	ctx := context.Background()
	schemaResp := datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	config := tfsdk.Config{Schema: schemaResp.Schema}
	state := tfsdk.State{Schema: schemaResp.Schema}
	model := SshKeyDataSourceModel{OrganizationName: types.StringValue("acme"), Name: types.StringValue("modules"),
		ID: types.StringNull(), PublicKey: types.StringNull(), Fingerprint: types.StringNull(), KnownHosts: types.StringNull()}
	require.Empty(t, state.Set(ctx, &model))
	config.Raw = state.Raw
	resp := datasource.ReadResponse{State: state}
	d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	var got SshKeyDataSourceModel
	require.Empty(t, resp.State.Get(ctx, &got))
	assert.Equal(t, "key-1", got.ID.ValueString())

	model.Name = types.StringValue("missing")
	require.Empty(t, state.Set(ctx, &model))
	config.Raw = state.Raw
	resp = datasource.ReadResponse{State: state}
	d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
	assert.True(t, resp.Diagnostics.HasError())
}

func TestWorkspace_SshKeyCanBeSetClearedOrLeftAlone(t *testing.T) {
	encode := func(req WorkspaceUpdateRequest) map[string]any {
		raw, err := json.Marshal(req)
		require.NoError(t, err)
		var out map[string]any
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}
	assert.NotContains(t, encode(WorkspaceUpdateRequest{Name: "ws"}), "ssh_key")
	assert.Equal(t, "key-1", encode(WorkspaceUpdateRequest{SshKey: nullableID(types.StringValue("key-1"))})["ssh_key"])
	cleared := encode(WorkspaceUpdateRequest{SshKey: nullableID(types.StringNull())})
	assert.Contains(t, cleared, "ssh_key")
	assert.Nil(t, cleared["ssh_key"])

	var data WorkspaceResourceModel
	mapWorkspaceResponseToModel(context.Background(), &data, WorkspaceAPIResponse{ID: "ws-1", Name: "ws"})
	assert.True(t, data.SshKeyID.IsNull())
	key := "key-1"
	mapWorkspaceResponseToModel(context.Background(), &data, WorkspaceAPIResponse{ID: "ws-1", Name: "ws", SshKey: &key})
	assert.Equal(t, "key-1", data.SshKeyID.ValueString())
}

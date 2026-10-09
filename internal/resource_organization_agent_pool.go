package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The organization's default agent pool, as its own resource: the pool belongs to the organization
// (infradots_worker_pool.organization_name), so setting it on infradots_organization itself would make
// the two depend on each other.
var (
	_ resource.Resource                = &OrganizationAgentPoolResource{}
	_ resource.ResourceWithConfigure   = &OrganizationAgentPoolResource{}
	_ resource.ResourceWithImportState = &OrganizationAgentPoolResource{}
)

func NewOrganizationAgentPoolResource() resource.Resource {
	return &OrganizationAgentPoolResource{}
}

type OrganizationAgentPoolResource struct {
	provider *InfradotsProvider
}

type OrganizationAgentPoolResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	AgentPoolID      types.String `tfsdk:"agent_pool_id"`
}

// organizationAgentPoolRequest sets (an id) or clears (nil, sent as null) the organization's agent pool.
type organizationAgentPoolRequest struct {
	AgentPool *string `json:"agent_pool"`
}

func (r *OrganizationAgentPoolResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_organization_agent_pool"
}

func (r *OrganizationAgentPoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The organization's agent pool: where its AI reviews and implementations run, on your own " +
			"runners, unless a workspace sets its own (`infradots_workspace.agent_pool_id`). Destroying this resource " +
			"sends the organization's agent runs back to InfraDots.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The organization name.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The organization.",
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"agent_pool_id": schema.StringAttribute{
				Description: "ID of one of the organization's agent pools (`infradots_worker_pool` with `kind = \"agent\"`).",
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
		},
	}
}

func (r *OrganizationAgentPoolResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
		r.provider = provider
	}
}

func (r *OrganizationAgentPoolResource) url(organizationName string) string {
	return fmt.Sprintf("https://%s/api/organizations/%s/", r.provider.host, organizationName)
}

// do sends a request about the organization and decodes the organization it returns. found is false on a 404.
func (r *OrganizationAgentPoolResource) do(method, organizationName string, body any) (org OrganizationAPIResponse, found bool, err error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return org, false, err
		}
		reader = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequest(method, r.url(organizationName), reader)
	if err != nil {
		return org, false, err
	}
	req.Header.Set("Authorization", "Bearer "+r.provider.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.provider.client.Do(req)
	if err != nil {
		return org, false, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return org, false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return org, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return org, false, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	return org, true, json.Unmarshal(respBody, &org)
}

// set points the organization at the configured pool and records what the API answered.
func (r *OrganizationAgentPoolResource) set(data *OrganizationAgentPoolResourceModel) error {
	poolID := data.AgentPoolID.ValueString()
	org, found, err := r.do(http.MethodPatch, data.OrganizationName.ValueString(), organizationAgentPoolRequest{AgentPool: &poolID})
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("organization %q not found", data.OrganizationName.ValueString())
	}
	data.ID = types.StringValue(data.OrganizationName.ValueString())
	data.AgentPoolID = types.StringPointerValue(org.AgentPool)
	return nil
}

func (r *OrganizationAgentPoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationAgentPoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.set(&data); err != nil {
		resp.Diagnostics.AddError("Error setting the organization's agent pool", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAgentPoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationAgentPoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	org, found, err := r.do(http.MethodGet, data.OrganizationName.ValueString(), nil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the organization's agent pool", err.Error())
		return
	}
	// The organization is gone, or its agent pool was cleared outside Terraform: nothing to manage.
	if !found || org.AgentPool == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.ID = types.StringValue(data.OrganizationName.ValueString())
	data.AgentPoolID = types.StringValue(*org.AgentPool)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAgentPoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationAgentPoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.set(&data); err != nil {
		resp.Diagnostics.AddError("Error setting the organization's agent pool", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAgentPoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationAgentPoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// A nil pool is sent as null: agent runs go back to InfraDots. A gone organization has nothing to clear.
	if _, _, err := r.do(http.MethodPatch, data.OrganizationName.ValueString(), organizationAgentPoolRequest{}); err != nil {
		resp.Diagnostics.AddError("Error clearing the organization's agent pool", err.Error())
	}
}

func (r *OrganizationAgentPoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("organization_name"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

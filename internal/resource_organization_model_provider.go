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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Which model provider the organization's agents use, as its own resource, for the same reason the
// agent pool is one: the provider belongs to the organization
// (infradots_model_provider.organization_name), so setting it on infradots_organization itself would
// make the two depend on each other.
var (
	_ resource.Resource                = &OrganizationModelProviderResource{}
	_ resource.ResourceWithConfigure   = &OrganizationModelProviderResource{}
	_ resource.ResourceWithImportState = &OrganizationModelProviderResource{}
)

func NewOrganizationModelProviderResource() resource.Resource {
	return &OrganizationModelProviderResource{}
}

type OrganizationModelProviderResource struct {
	provider *InfradotsProvider
}

type OrganizationModelProviderResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	OrganizationName      types.String `tfsdk:"organization_name"`
	ModelProviderID       types.String `tfsdk:"model_provider_id"`
	AllowPlatformFallback types.Bool   `tfsdk:"allow_platform_fallback"`
}

// organizationModelProviderRequest sets (an id) or clears (nil, sent as null) the organization's
// model provider, and carries the fallback choice that goes with it.
type organizationModelProviderRequest struct {
	ModelProvider         *string `json:"model_provider"`
	AllowPlatformFallback *bool   `json:"allow_platform_fallback,omitempty"`
}

func (r *OrganizationModelProviderResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_organization_model_provider"
}

func (r *OrganizationModelProviderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The model provider the organization's agents use: the same Claude models InfraDots runs, " +
			"billed to your own account (`infradots_model_provider`). Destroying this resource sends the organization's " +
			"model calls back to InfraDots' own account.\n\n" +
			"Runs on your own agent pool are routed by the runner itself, from its `MODEL_PROVIDER` environment, since " +
			"a runner holds its own credentials. Pointing an organization here still has an effect for those runs: " +
			"InfraDots checks that each runner sends its calls where you said, and reports a runner that doesn't.",
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
			"model_provider_id": schema.StringAttribute{
				Description: "ID of one of the organization's model providers (`infradots_model_provider`).",
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"allow_platform_fallback": schema.BoolAttribute{
				Description: "When the provider cannot be reached, run on InfraDots' model instead of failing the run. " +
					"Defaults to false, so a broken key cannot quietly move the model bill back to InfraDots.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
		},
	}
}

func (r *OrganizationModelProviderResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
		r.provider = provider
	}
}

func (r *OrganizationModelProviderResource) url(organizationName string) string {
	return fmt.Sprintf("https://%s/api/organizations/%s/", r.provider.host, organizationName)
}

// do sends a request about the organization and decodes the organization it returns. found is false on a 404.
func (r *OrganizationModelProviderResource) do(method, organizationName string, body any) (org OrganizationAPIResponse, found bool, err error) {
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

// set points the organization at the configured provider and records what the API answered.
func (r *OrganizationModelProviderResource) set(data *OrganizationModelProviderResourceModel) error {
	providerID := data.ModelProviderID.ValueString()
	fallback := data.AllowPlatformFallback.ValueBool()
	org, found, err := r.do(http.MethodPatch, data.OrganizationName.ValueString(),
		organizationModelProviderRequest{ModelProvider: &providerID, AllowPlatformFallback: &fallback})
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("organization %q not found", data.OrganizationName.ValueString())
	}
	data.ID = types.StringValue(data.OrganizationName.ValueString())
	data.ModelProviderID = types.StringPointerValue(org.ModelProvider)
	data.AllowPlatformFallback = types.BoolValue(org.AllowPlatformFallback)
	return nil
}

func (r *OrganizationModelProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationModelProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.set(&data); err != nil {
		resp.Diagnostics.AddError("Error setting the organization's model provider", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationModelProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationModelProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	org, found, err := r.do(http.MethodGet, data.OrganizationName.ValueString(), nil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the organization's model provider", err.Error())
		return
	}
	// The organization is gone, or its model provider was cleared outside Terraform: nothing to manage.
	if !found || org.ModelProvider == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.ID = types.StringValue(data.OrganizationName.ValueString())
	data.ModelProviderID = types.StringValue(*org.ModelProvider)
	data.AllowPlatformFallback = types.BoolValue(org.AllowPlatformFallback)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationModelProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationModelProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.set(&data); err != nil {
		resp.Diagnostics.AddError("Error setting the organization's model provider", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationModelProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationModelProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// A nil provider is sent as null: model calls go back to InfraDots' own account, and the
	// fallback setting goes back to its default, since there is no longer anything to fall back from.
	fallback := false
	if _, _, err := r.do(http.MethodPatch, data.OrganizationName.ValueString(),
		organizationModelProviderRequest{AllowPlatformFallback: &fallback}); err != nil {
		resp.Diagnostics.AddError("Error clearing the organization's model provider", err.Error())
	}
}

func (r *OrganizationModelProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("organization_name"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

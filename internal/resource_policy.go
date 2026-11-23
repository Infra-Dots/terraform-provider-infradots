package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &PolicyResource{}
var _ resource.ResourceWithImportState = &PolicyResource{}
var _ resource.ResourceWithValidateConfig = &PolicyResource{}

// The API strips surrounding spaces from list entries; rejecting them here keeps state equal to config.
var policyStrings = []validator.List{
	listvalidator.SizeBetween(1, 100),
	listvalidator.ValueStringsAre(stringvalidator.RegexMatches(regexp.MustCompile(`^\S(.*\S)?$`),
		"must be non-empty, without leading or trailing spaces")),
}

func NewPolicyResource() resource.Resource {
	return &PolicyResource{}
}

// PolicyResource is an organization's plan policy, checked against every plan of the workspaces it
// applies to. The API keeps a kind's settings in one `config` object; here they are typed attributes,
// so a plan shows which setting changed.
type PolicyResource struct {
	provider *InfradotsProvider
}

type PolicyResourceModel struct {
	ID               types.String  `tfsdk:"id"`
	OrganizationName types.String  `tfsdk:"organization_name"`
	Name             types.String  `tfsdk:"name"`
	Description      types.String  `tfsdk:"description"`
	Kind             types.String  `tfsdk:"kind"`
	Enforcement      types.String  `tfsdk:"enforcement"`
	WorkspaceTags    types.Map     `tfsdk:"workspace_tags"`
	ResourceType     types.String  `tfsdk:"resource_type"`
	Path             types.String  `tfsdk:"path"`
	Op               types.String  `tfsdk:"op"`
	Value            types.Dynamic `tfsdk:"value"`
	Values           types.List    `tfsdk:"values"`
	RequiredTags     types.List    `tfsdk:"required_tags"`
	Ports            types.Set     `tfsdk:"ports"`
}

type PolicyAPIResponse struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Kind          string            `json:"kind"`
	Config        map[string]any    `json:"config"`
	Enforcement   string            `json:"enforcement"`
	WorkspaceTags map[string]string `json:"workspace_tags"`
}

type PolicyRequest struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Kind          string            `json:"kind,omitempty"` // create only: a policy's kind can't change
	Config        map[string]any    `json:"config"`
	Enforcement   string            `json:"enforcement"`
	WorkspaceTags map[string]string `json:"workspace_tags"`
}

// policyKindAttributes are the settings each kind reads; setting another kind's is a config error.
var policyKindAttributes = map[string][]string{
	"attribute":       {"resource_type", "path", "op", "value", "values"},
	"require_tags":    {"required_tags"},
	"no_open_ingress": {"ports"},
	"no_delete":       {},
}

func (r *PolicyResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_policy"
}

func (r *PolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A plan policy: a rule InfraDots checks against every plan of the workspaces it applies to. " +
			"A blocking policy with violations fails the run before apply.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The policy UUID.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_name": schema.StringAttribute{
				Description:   "The organization the policy belongs to.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "The policy's name, unique in the organization. Violations are reported under it.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "What the policy is for.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"kind": schema.StringAttribute{
				Description:   "What the policy checks: attribute, require_tags, no_open_ingress or no_delete. Changing it forces a new policy.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.OneOf("attribute", "require_tags", "no_open_ingress", "no_delete")},
			},
			"enforcement": schema.StringAttribute{
				Description: "off, advisory (report only) or blocking (fail the run before apply). Defaults to advisory.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("advisory"),
				Validators:  []validator.String{stringvalidator.OneOf("off", "advisory", "blocking")},
			},
			"workspace_tags": schema.MapAttribute{
				Description: "The policy applies to workspaces carrying every one of these tags. Empty means every workspace.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
			},
			"resource_type": schema.StringAttribute{
				Description: "attribute: the resource type to check, such as aws_s3_bucket, or * for every resource that has the attribute.",
				Optional:    true,
			},
			"path": schema.StringAttribute{
				Description: "attribute: the dotted attribute path, such as acl or tags.owner.",
				Optional:    true,
			},
			"op": schema.StringAttribute{
				Description: "attribute: eq, in, not_in or exists.",
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf("eq", "in", "not_in", "exists")},
			},
			"value": schema.DynamicAttribute{
				Description: "attribute with op = eq: the string, number or bool the attribute must equal.",
				Optional:    true,
			},
			"values": schema.ListAttribute{
				Description: "attribute with op = in or not_in: the values the attribute must, or must not, be one of.",
				ElementType: types.StringType,
				Optional:    true,
				Validators:  policyStrings,
			},
			"required_tags": schema.ListAttribute{
				Description: "require_tags: the tag keys every taggable resource must carry. Provider default_tags count.",
				ElementType: types.StringType,
				Optional:    true,
				Validators:  policyStrings,
			},
			"ports": schema.SetAttribute{
				Description: "no_open_ingress: the ports a security group must not open to 0.0.0.0/0.",
				ElementType: types.Int64Type,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeBetween(1, 100),
					setvalidator.ValueInt64sAre(int64validator.Between(0, 65535)),
				},
			},
		},
	}
}

func (r *PolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.provider = req.ProviderData.(*InfradotsProvider)
	}
}

// ValidateConfig catches a kind's missing or foreign settings at plan time; the API would reject them
// only on apply.
func (r *PolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Kind.IsUnknown() || data.Kind.IsNull() {
		return
	}
	kind := data.Kind.ValueString()
	set := map[string]attr.Value{
		"resource_type": data.ResourceType, "path": data.Path, "op": data.Op, "value": data.Value,
		"values": data.Values, "required_tags": data.RequiredTags, "ports": data.Ports,
	}
	allowed := map[string]bool{}
	for _, name := range policyKindAttributes[kind] {
		allowed[name] = true
	}
	for name, value := range set {
		if !value.IsNull() && !allowed[name] {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Setting not used by this kind",
				fmt.Sprintf("%s is not a setting of the %s kind.", name, kind))
		}
	}
	require := func(name string) {
		if set[name].IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Missing setting",
				fmt.Sprintf("The %s kind needs %s.", kind, name))
		}
	}
	switch kind {
	case "attribute":
		require("resource_type")
		require("path")
		require("op")
		if data.Op.IsUnknown() || data.Op.IsNull() {
			return
		}
		op := data.Op.ValueString()
		if op == "eq" {
			require("value")
		} else if !data.Value.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("value"), "Setting not used by this op", "value is only read by op = eq.")
		}
		if op == "in" || op == "not_in" {
			require("values")
		} else if !data.Values.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("values"), "Setting not used by this op", "values is only read by op = in or not_in.")
		}
	case "require_tags":
		require("required_tags")
	case "no_open_ingress":
		require("ports")
	}
}

func (r *PolicyResource) url(org, id string) string {
	base := fmt.Sprintf("https://%s/api/organizations/%s/policies/", r.provider.host, org)
	if id == "" {
		return base
	}
	return base + id + "/"
}

// call sends a request and decodes the policy in the response; found is false on a 404.
func (r *PolicyResource) call(method, url string, body any, okStatus int) (policy PolicyAPIResponse, found bool, err error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return policy, false, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return policy, false, err
	}
	req.Header.Set("Authorization", "Bearer "+r.provider.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.provider.client.Do(req)
	if err != nil {
		return policy, false, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return policy, false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return policy, false, nil
	}
	if resp.StatusCode != okStatus {
		return policy, false, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	if method == http.MethodDelete {
		return policy, true, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(respBody))
	decoder.UseNumber() // keeps an eq value of 5 a number, not 5.0
	return policy, true, decoder.Decode(&policy)
}

// request builds the API body from the model: the typed settings go back into the kind's `config`.
func (m *PolicyResourceModel) request(ctx context.Context) (PolicyRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	config := map[string]any{}
	switch m.Kind.ValueString() {
	case "attribute":
		config["resource_type"] = m.ResourceType.ValueString()
		config["path"] = m.Path.ValueString()
		config["op"] = m.Op.ValueString()
		if !m.Value.IsNull() {
			value, err := dynamicToJSON(m.Value)
			if err != nil {
				diags.AddAttributeError(path.Root("value"), "Invalid value", err.Error())
			}
			config["value"] = value
		}
		if !m.Values.IsNull() {
			var values []string
			diags.Append(m.Values.ElementsAs(ctx, &values, false)...)
			config["value"] = values
		}
	case "require_tags":
		var tags []string
		diags.Append(m.RequiredTags.ElementsAs(ctx, &tags, false)...)
		config["tags"] = tags
	case "no_open_ingress":
		var ports []int64
		diags.Append(m.Ports.ElementsAs(ctx, &ports, false)...)
		config["ports"] = ports
	}
	workspaceTags := map[string]string{}
	if !m.WorkspaceTags.IsNull() && !m.WorkspaceTags.IsUnknown() {
		diags.Append(m.WorkspaceTags.ElementsAs(ctx, &workspaceTags, false)...)
	}
	return PolicyRequest{
		Name: m.Name.ValueString(), Description: m.Description.ValueString(), Config: config,
		Enforcement: m.Enforcement.ValueString(), WorkspaceTags: workspaceTags,
	}, diags
}

// apply copies what the API returned into the model. Settings the policy's kind doesn't read are null.
func (m *PolicyResourceModel) apply(p PolicyAPIResponse) error {
	m.ID = types.StringValue(p.ID)
	m.Name = types.StringValue(p.Name)
	m.Description = types.StringValue(p.Description)
	m.Kind = types.StringValue(p.Kind)
	m.Enforcement = types.StringValue(p.Enforcement)
	m.WorkspaceTags = mapValue(p.WorkspaceTags)
	m.ResourceType, m.Path, m.Op = types.StringNull(), types.StringNull(), types.StringNull()
	m.Value = types.DynamicNull()
	m.Values = types.ListNull(types.StringType)
	m.RequiredTags = types.ListNull(types.StringType)
	m.Ports = types.SetNull(types.Int64Type)
	c := p.Config
	switch p.Kind {
	case "attribute":
		m.ResourceType = types.StringValue(fmt.Sprint(c["resource_type"]))
		m.Path = types.StringValue(fmt.Sprint(c["path"]))
		m.Op = types.StringValue(fmt.Sprint(c["op"]))
		switch value := c["value"].(type) {
		case nil:
		case []any:
			m.Values = jsonStringList(value)
		default:
			dynamic, err := jsonToDynamic(value)
			if err != nil {
				return err
			}
			m.Value = dynamic
		}
	case "require_tags":
		tags, _ := c["tags"].([]any)
		m.RequiredTags = jsonStringList(tags)
	case "no_open_ingress":
		ports, _ := c["ports"].([]any)
		elements := make([]attr.Value, 0, len(ports))
		for _, port := range ports {
			n, err := port.(json.Number).Int64()
			if err != nil {
				return fmt.Errorf("port %v: %w", port, err)
			}
			elements = append(elements, types.Int64Value(n))
		}
		m.Ports = types.SetValueMust(types.Int64Type, elements)
	}
	return nil
}

func jsonStringList(values []any) types.List {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(fmt.Sprint(value)))
	}
	return types.ListValueMust(types.StringType, elements)
}

// dynamicToJSON and jsonToDynamic carry an eq value across with its type: OPA compares 5, "5" and
// true exactly, so a string where the plan has a number never matches.
func dynamicToJSON(value types.Dynamic) (any, error) {
	switch v := value.UnderlyingValue().(type) {
	case types.String:
		return v.ValueString(), nil
	case types.Bool:
		return v.ValueBool(), nil
	case types.Number:
		return json.Number(v.ValueBigFloat().Text('g', -1)), nil
	}
	return nil, fmt.Errorf("value must be a string, number or bool, not %s", value.UnderlyingValue().Type(context.Background()))
}

func jsonToDynamic(value any) (types.Dynamic, error) {
	switch v := value.(type) {
	case string:
		return types.DynamicValue(types.StringValue(v)), nil
	case bool:
		return types.DynamicValue(types.BoolValue(v)), nil
	case json.Number:
		n, ok := new(big.Float).SetString(v.String())
		if !ok {
			return types.DynamicNull(), fmt.Errorf("value %q is not a number", v)
		}
		return types.DynamicValue(types.NumberValue(n)), nil
	}
	return types.DynamicNull(), fmt.Errorf("value %v is not a string, number or bool", value)
}

func (r *PolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := data.request(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Kind = data.Kind.ValueString()
	policy, _, err := r.call(http.MethodPost, r.url(data.OrganizationName.ValueString(), ""), body, http.StatusCreated)
	if err == nil {
		err = data.apply(policy)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error creating policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policy, found, err := r.call(http.MethodGet, r.url(data.OrganizationName.ValueString(), data.ID.ValueString()), nil, http.StatusOK)
	if err == nil && !found {
		resp.State.RemoveResource(ctx)
		return
	}
	if err == nil {
		err = data.apply(policy)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state PolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := plan.request(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	policy, found, err := r.call(http.MethodPatch, r.url(state.OrganizationName.ValueString(), state.ID.ValueString()), body, http.StatusOK)
	if err == nil && !found {
		err = fmt.Errorf("policy %s no longer exists", state.ID.ValueString())
	}
	if err == nil {
		err = plan.apply(policy)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, _, err := r.call(http.MethodDelete, r.url(data.OrganizationName.ValueString(), data.ID.ValueString()), nil, http.StatusNoContent)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting policy", err.Error())
	}
}

// ImportState takes `organization_name:id`.
func (r *PolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Use the format 'organization_name:id'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

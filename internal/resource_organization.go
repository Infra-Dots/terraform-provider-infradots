package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource              = &OrganizationResource{}
	_ resource.ResourceWithConfigure = &OrganizationResource{}
)

func NewOrganizationResource() resource.Resource {
	return &OrganizationResource{}
}

type OrganizationResourceModel struct {
	ID                           types.String `tfsdk:"id"`
	Name                         types.String `tfsdk:"name"`
	CreatedAt                    types.String `tfsdk:"created_at"`
	UpdatedAt                    types.String `tfsdk:"updated_at"`
	ExecutionMode                types.String `tfsdk:"execution_mode"` // execution mode (Remote, Local)
	AgentsEnabled                types.Bool   `tfsdk:"agents_enabled"` // boolean indicating if IDP agents are enabled
	Tags                         types.Map    `tfsdk:"tags"`
	DriftDetectionEnabled        types.Bool   `tfsdk:"drift_detection_enabled"`
	RemedyDrift                  types.Bool   `tfsdk:"remedy_drift"`
	AutoImplementChanges         types.Bool   `tfsdk:"auto_implement_changes"`
	ApprovalReminderIntervalHours types.Int64  `tfsdk:"approval_reminder_interval_hours"`
}

type OrganizationAPIResponse struct {
	ID                           string         `json:"id"`
	Name                         string         `json:"name"`
	Members                      []Member       `json:"members"`
	CreatedAt                    time.Time      `json:"created_at"`
	UpdatedAt                    time.Time      `json:"updated_at"`
	Subscription                 map[string]any `json:"subscription"`
	Tags                         map[string]any `json:"tags"`
	Teams                        []Team         `json:"teams"`
	ExecutionMode                string         `json:"execution_mode"`
	AgentsEnabled                bool           `json:"agents_enabled"`
	DriftDetectionEnabled        bool           `json:"drift_detection_enabled"`
	RemedyDrift                  bool           `json:"remedy_drift"`
	AutoImplementChanges         bool           `json:"auto_implement_changes"`
	ApprovalReminderIntervalHours *int64         `json:"approval_reminder_interval_hours"`
}

type Member struct {
	Email string `json:"email"`
}

type Team struct {
	Name string `json:"name"`
}

type OrganizationUpdateRequest struct {
	Name                         string         `json:"name,omitempty"`
	ExecutionMode                string         `json:"execution_mode,omitempty"`
	AgentsEnabled                bool           `json:"agents_enabled,omitempty"`
	Tags                         map[string]any `json:"tags,omitempty"`
	DriftDetectionEnabled        *bool          `json:"drift_detection_enabled,omitempty"`
	RemedyDrift                  *bool          `json:"remedy_drift,omitempty"`
	AutoImplementChanges         *bool          `json:"auto_implement_changes,omitempty"`
	ApprovalReminderIntervalHours *int64         `json:"approval_reminder_interval_hours,omitempty"`
}

type OrganizationResource struct {
	provider *InfradotsProvider
}

func (r *OrganizationResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_organization"
}

func (r *OrganizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Organization in InfraDots Platform",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The organization unique ID (UUID).",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The unique name of the organization.",
				Required:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the organization was created.",
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the organization was last updated.",
				Computed:    true,
			},
			"execution_mode": schema.StringAttribute{
				Description: "The execution mode for the organization (Remote, Local, etc.).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("remote"),
				Validators: []validator.String{
					stringvalidator.OneOf("local", "remote"),
				},
			},
			"agents_enabled": schema.BoolAttribute{
				Description: "Whether IDP agents are enabled for the organization.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"tags": schema.MapAttribute{
				Description: "Tags for the organization.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
			},
			"drift_detection_enabled": schema.BoolAttribute{
				Description: "Whether drift detection is enabled for the organization.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"remedy_drift": schema.BoolAttribute{
				Description: "Whether to automatically remedy detected drift.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"auto_implement_changes": schema.BoolAttribute{
				Description: "Whether to automatically implement changes.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"approval_reminder_interval_hours": schema.Int64Attribute{
				Description: "How often (in hours) to send approval reminder notifications for jobs pending approval. Defaults to 1. Set to null to disable reminders.",
				Optional:    true,
			},
		},
	}
}

func (r *OrganizationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *OrganizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	driftDetection := data.DriftDetectionEnabled.ValueBool()
	remedyDrift := data.RemedyDrift.ValueBool()
	autoImplement := data.AutoImplementChanges.ValueBool()

	createReq := OrganizationUpdateRequest{
		Name:                  data.Name.ValueString(),
		ExecutionMode:         data.ExecutionMode.ValueString(),
		AgentsEnabled:         data.AgentsEnabled.ValueBool(),
		DriftDetectionEnabled: &driftDetection,
		RemedyDrift:           &remedyDrift,
		AutoImplementChanges:  &autoImplement,
	}

	if !data.ApprovalReminderIntervalHours.IsNull() && !data.ApprovalReminderIntervalHours.IsUnknown() {
		v := data.ApprovalReminderIntervalHours.ValueInt64()
		createReq.ApprovalReminderIntervalHours = &v
	}

	if !data.Tags.IsNull() && !data.Tags.IsUnknown() {
		var tags map[string]string
		diags = data.Tags.ElementsAs(ctx, &tags, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		tagsAny := map[string]any{}
		for k, v := range tags {
			tagsAny[k] = v
		}
		createReq.Tags = tagsAny
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/", r.provider.host)

	reqHttp, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)
	reqHttp.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("Couldn't create Infradots organization", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	if httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError(
			"Non-201 response",
			fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	var organization OrganizationAPIResponse
	err = json.Unmarshal(respBody, &organization)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(organization.ID)
	data.CreatedAt = types.StringValue(organization.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(organization.UpdatedAt.Format(time.RFC3339))
	data.ExecutionMode = types.StringValue(strings.ToLower(organization.ExecutionMode))
	data.AgentsEnabled = types.BoolValue(organization.AgentsEnabled)
	data.DriftDetectionEnabled = types.BoolValue(organization.DriftDetectionEnabled)
	data.RemedyDrift = types.BoolValue(organization.RemedyDrift)
	data.AutoImplementChanges = types.BoolValue(organization.AutoImplementChanges)

	if organization.ApprovalReminderIntervalHours != nil {
		data.ApprovalReminderIntervalHours = types.Int64Value(*organization.ApprovalReminderIntervalHours)
	} else {
		data.ApprovalReminderIntervalHours = types.Int64Null()
	}

	if organization.Tags != nil {
		tagMap := map[string]attr.Value{}
		for k, v := range organization.Tags {
			tagMap[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		data.Tags = types.MapValueMust(types.StringType, tagMap)
	} else {
		data.Tags = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	diags = resp.State.Set(ctx, &data)
	tflog.Info(ctx, "Module Resource Created", map[string]any{"success": true})
	resp.Diagnostics.Append(diags...)
}

func (r *OrganizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/", r.provider.host, data.ID.ValueString())

	reqHttp, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var organization OrganizationAPIResponse
	err = json.Unmarshal(respBody, &organization)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(organization.ID)
	data.Name = types.StringValue(organization.Name)
	data.CreatedAt = types.StringValue(organization.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(organization.UpdatedAt.Format(time.RFC3339))
	data.ExecutionMode = types.StringValue(organization.ExecutionMode)
	data.AgentsEnabled = types.BoolValue(organization.AgentsEnabled)
	data.DriftDetectionEnabled = types.BoolValue(organization.DriftDetectionEnabled)
	data.RemedyDrift = types.BoolValue(organization.RemedyDrift)
	data.AutoImplementChanges = types.BoolValue(organization.AutoImplementChanges)

	if organization.ApprovalReminderIntervalHours != nil {
		data.ApprovalReminderIntervalHours = types.Int64Value(*organization.ApprovalReminderIntervalHours)
	} else {
		data.ApprovalReminderIntervalHours = types.Int64Null()
	}

	if organization.Tags != nil {
		tagMap := map[string]attr.Value{}
		for k, v := range organization.Tags {
			tagMap[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		data.Tags = types.MapValueMust(types.StringType, tagMap)
	} else {
		data.Tags = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *OrganizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state OrganizationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan OrganizationResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := OrganizationUpdateRequest{}

	if !plan.Name.Equal(state.Name) {
		updateReq.Name = plan.Name.ValueString()
	}

	if !plan.ExecutionMode.Equal(state.ExecutionMode) {
		updateReq.ExecutionMode = plan.ExecutionMode.ValueString()
	}

	if !plan.AgentsEnabled.Equal(state.AgentsEnabled) {
		updateReq.AgentsEnabled = plan.AgentsEnabled.ValueBool()
	}

	if !plan.DriftDetectionEnabled.Equal(state.DriftDetectionEnabled) {
		v := plan.DriftDetectionEnabled.ValueBool()
		updateReq.DriftDetectionEnabled = &v
	}

	if !plan.RemedyDrift.Equal(state.RemedyDrift) {
		v := plan.RemedyDrift.ValueBool()
		updateReq.RemedyDrift = &v
	}

	if !plan.AutoImplementChanges.Equal(state.AutoImplementChanges) {
		v := plan.AutoImplementChanges.ValueBool()
		updateReq.AutoImplementChanges = &v
	}

	if !plan.ApprovalReminderIntervalHours.Equal(state.ApprovalReminderIntervalHours) {
		if !plan.ApprovalReminderIntervalHours.IsNull() && !plan.ApprovalReminderIntervalHours.IsUnknown() {
			v := plan.ApprovalReminderIntervalHours.ValueInt64()
			updateReq.ApprovalReminderIntervalHours = &v
		}
	}

	if !plan.Tags.Equal(state.Tags) && !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		var tags map[string]string
		diags = plan.Tags.ElementsAs(ctx, &tags, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		tagsAny := map[string]any{}
		for k, v := range tags {
			tagsAny[k] = v
		}
		updateReq.Tags = tagsAny
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/", r.provider.host, plan.ID.ValueString())

	reqHttp, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)
	reqHttp.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Update failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	var organization OrganizationAPIResponse
	err = json.Unmarshal(respBody, &organization)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(organization.ID)
	plan.Name = types.StringValue(organization.Name)
	plan.CreatedAt = types.StringValue(organization.CreatedAt.Format(time.RFC3339))
	plan.UpdatedAt = types.StringValue(organization.UpdatedAt.Format(time.RFC3339))
	plan.ExecutionMode = types.StringValue(organization.ExecutionMode)
	plan.AgentsEnabled = types.BoolValue(organization.AgentsEnabled)
	plan.DriftDetectionEnabled = types.BoolValue(organization.DriftDetectionEnabled)
	plan.RemedyDrift = types.BoolValue(organization.RemedyDrift)
	plan.AutoImplementChanges = types.BoolValue(organization.AutoImplementChanges)

	if organization.ApprovalReminderIntervalHours != nil {
		plan.ApprovalReminderIntervalHours = types.Int64Value(*organization.ApprovalReminderIntervalHours)
	} else {
		plan.ApprovalReminderIntervalHours = types.Int64Null()
	}

	if organization.Tags != nil {
		tagMap := map[string]attr.Value{}
		for k, v := range organization.Tags {
			tagMap[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		plan.Tags = types.MapValueMust(types.StringType, tagMap)
	} else {
		plan.Tags = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *OrganizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/", r.provider.host, data.ID.ValueString())

	reqHttp, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Delete failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *OrganizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	organizationName := req.ID

	url := fmt.Sprintf("https://%s/api/organizations/%s/", r.provider.host, organizationName)

	reqHttp, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	// If 404, organization not found
	if httpResp.StatusCode == 404 {
		resp.Diagnostics.AddError(
			"Organization not found",
			fmt.Sprintf("Organization '%s' not found", organizationName),
		)
		return
	}

	if httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"Failed to fetch organization",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var organization OrganizationAPIResponse
	err = json.Unmarshal(respBody, &organization)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var data OrganizationResourceModel
	data.ID = types.StringValue(organization.ID)
	data.Name = types.StringValue(organization.Name)
	data.CreatedAt = types.StringValue(organization.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(organization.UpdatedAt.Format(time.RFC3339))
	data.ExecutionMode = types.StringValue(strings.ToLower(organization.ExecutionMode))
	data.AgentsEnabled = types.BoolValue(organization.AgentsEnabled)
	data.DriftDetectionEnabled = types.BoolValue(organization.DriftDetectionEnabled)
	data.RemedyDrift = types.BoolValue(organization.RemedyDrift)
	data.AutoImplementChanges = types.BoolValue(organization.AutoImplementChanges)

	if organization.ApprovalReminderIntervalHours != nil {
		data.ApprovalReminderIntervalHours = types.Int64Value(*organization.ApprovalReminderIntervalHours)
	} else {
		data.ApprovalReminderIntervalHours = types.Int64Null()
	}

	if organization.Tags != nil {
		tagMap := map[string]attr.Value{}
		for k, v := range organization.Tags {
			tagMap[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		data.Tags = types.MapValueMust(types.StringType, tagMap)
	} else {
		data.Tags = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure we fully satisfy the resource.Resource interface.
var _ resource.Resource = &WorkspaceResource{}

func NewWorkspaceResource() resource.Resource {
	return &WorkspaceResource{}
}

type WorkspaceResourceModel struct {
	ID                    types.String `tfsdk:"id"`                // UUID
	OrganizationName      types.String `tfsdk:"organization_name"` // Name of the organization
	Name                  types.String `tfsdk:"name"`
	Description           types.String `tfsdk:"description"`
	Source                types.String `tfsdk:"source"`
	Branch                types.String `tfsdk:"branch"`
	TerraformVersion      types.String `tfsdk:"terraform_version"`
	CreatedAt             types.String `tfsdk:"created_at"` // timestamp
	UpdatedAt             types.String `tfsdk:"updated_at"` // timestamp
	VcsId                 types.String `tfsdk:"vcs_id"`     // UUID of a VCS provider in IDP
	VCS                   types.Object `tfsdk:"vcs"`        // VCS object as returned by API
	Locked                types.Bool   `tfsdk:"locked"`
	AutoApply             types.Bool   `tfsdk:"auto_apply"`
	IacType               types.String `tfsdk:"iac_type"`
	DefaultJobAction      types.String `tfsdk:"default_job_action"`
	WorkerPoolID          types.String `tfsdk:"worker_pool_id"`
	Folder                types.String `tfsdk:"folder"`
	ExecutionMode         types.String `tfsdk:"execution_mode"`
	Tags                  types.Map    `tfsdk:"tags"`
	AgentsEnabled         types.Bool   `tfsdk:"agents_enabled"`
	DriftDetectionEnabled types.Bool   `tfsdk:"drift_detection_enabled"`
	RemedyDrift           types.Bool   `tfsdk:"remedy_drift"`
	AutoImplementChanges  types.Bool   `tfsdk:"auto_implement_changes"`
	SshId                 types.String `tfsdk:"ssh_id"`
	ModuleSshKey          types.String `tfsdk:"module_ssh_key"`
}

type WorkspaceAPIResponse struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Description           string          `json:"description"`
	Source                string          `json:"source"`
	Branch                string          `json:"branch"`
	TerraformVersion      string          `json:"terraform_version"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	VCS                   *VCSAPIResponse `json:"vcs"`
	Locked                bool            `json:"locked"`
	AutoApply             bool            `json:"auto_apply"`
	IacType               string          `json:"iac_type"`
	DefaultJobAction      string          `json:"default_job_action"`
	WorkerPool            *string         `json:"worker_pool"`
	Folder                string          `json:"folder"`
	ExecutionMode         string          `json:"execution_mode"`
	Tags                  map[string]any  `json:"tags"`
	AgentsEnabled         bool            `json:"agents_enabled"`
	DriftDetectionEnabled *bool           `json:"drift_detection_enabled"`
	RemedyDrift           *bool           `json:"remedy_drift"`
	AutoImplementChanges  *bool           `json:"auto_implement_changes"`
	SshId                 string          `json:"ssh_id"`
	ModuleSshKey          string          `json:"module_ssh_key"`
}

type WorkspaceCreateRequest struct {
	Name                  string         `json:"name"`
	Description           string         `json:"description,omitempty"`
	Source                string         `json:"source"`
	Branch                string         `json:"branch"`
	TerraformVersion      string         `json:"terraform_version"`
	AutoApply             bool           `json:"auto_apply"`
	IacType               string         `json:"iac_type,omitempty"`
	DefaultJobAction      string         `json:"default_job_action,omitempty"`
	WorkerPool            string         `json:"worker_pool,omitempty"`
	Folder                string         `json:"folder,omitempty"`
	ExecutionMode         string         `json:"execution_mode,omitempty"`
	Tags                  map[string]any `json:"tags,omitempty"`
	AgentsEnabled         bool           `json:"agents_enabled"`
	DriftDetectionEnabled *bool          `json:"drift_detection_enabled,omitempty"`
	RemedyDrift           *bool          `json:"remedy_drift,omitempty"`
	AutoImplementChanges  *bool          `json:"auto_implement_changes,omitempty"`
	SshId                 string         `json:"ssh_id,omitempty"`
	ModuleSshKey          string         `json:"module_ssh_key,omitempty"`
}

type WorkspaceUpdateRequest struct {
	Name                  string         `json:"name,omitempty"`
	Description           string         `json:"description,omitempty"`
	Source                string         `json:"source,omitempty"`
	Branch                string         `json:"branch,omitempty"`
	TerraformVersion      string         `json:"terraform_version,omitempty"`
	AutoApply             *bool          `json:"auto_apply,omitempty"`
	IacType               string         `json:"iac_type,omitempty"`
	DefaultJobAction      string         `json:"default_job_action,omitempty"`
	WorkerPool            string         `json:"worker_pool,omitempty"`
	Folder                string         `json:"folder,omitempty"`
	ExecutionMode         string         `json:"execution_mode,omitempty"`
	Tags                  map[string]any `json:"tags,omitempty"`
	AgentsEnabled         *bool          `json:"agents_enabled,omitempty"`
	DriftDetectionEnabled *bool          `json:"drift_detection_enabled,omitempty"`
	RemedyDrift           *bool          `json:"remedy_drift,omitempty"`
	AutoImplementChanges  *bool          `json:"auto_implement_changes,omitempty"`
	SshId                 string         `json:"ssh_id,omitempty"`
	ModuleSshKey          string         `json:"module_ssh_key,omitempty"`
}

type WorkspaceResource struct {
	provider *InfradotsProvider
}

func (r *WorkspaceResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_workspace"
}

func (r *WorkspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The workspace unique ID (UUID).",
				Computed:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this workspace belongs to.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the workspace.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "A short description of the workspace.",
				Optional:    true,
			},
			"source": schema.StringAttribute{
				Description: "Source repository URL or path.",
				Required:    true,
			},
			"branch": schema.StringAttribute{
				Description: "Git branch to use (if applicable).",
				Required:    true,
			},
			"terraform_version": schema.StringAttribute{
				Description: "Terraform version to use.",
				Required:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the workspace was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "The timestamp when the workspace was last updated.",
				Computed:    true,
			},
			"vcs_id": schema.StringAttribute{
				Description: "ID of a VCS Provider in infradots to connect to the workspace",
				Required:    false,
				Optional:    true,
			},
			"locked": schema.BoolAttribute{
				Description: "Whether the workspace is locked.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"auto_apply": schema.BoolAttribute{
				Description: "Whether to auto-apply successful plans.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"iac_type": schema.StringAttribute{
				Description: "The IaC type: TF (terraform), OT (opentofu), or TG (terragrunt).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("TF"),
				Validators: []validator.String{
					stringvalidator.OneOf("TF", "OT", "TG"),
				},
			},
			"default_job_action": schema.StringAttribute{
				Description: "Default job action: plan, apply, destroy, or refresh.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("plan"),
				Validators: []validator.String{
					stringvalidator.OneOf("plan", "apply", "destroy", "refresh"),
				},
			},
			"worker_pool_id": schema.StringAttribute{
				Description: "ID of the worker pool to assign to this workspace.",
				Optional:    true,
			},
			"folder": schema.StringAttribute{
				Description: "The subfolder within the source repository.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("/"),
			},
			"execution_mode": schema.StringAttribute{
				Description: "Execution mode for the workspace: Local or Remote.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Remote"),
				Validators: []validator.String{
					stringvalidator.OneOf("Local", "Remote"),
				},
			},
			"tags": schema.MapAttribute{
				Description: "Tags for the workspace.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"agents_enabled": schema.BoolAttribute{
				Description: "Whether AI agents are enabled for this workspace.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"drift_detection_enabled": schema.BoolAttribute{
				Description: "Whether drift detection is enabled. If null, inherits from the organization.",
				Optional:    true,
			},
			"remedy_drift": schema.BoolAttribute{
				Description: "Whether to remedy drift automatically. If null, inherits from the organization.",
				Optional:    true,
			},
			"auto_implement_changes": schema.BoolAttribute{
				Description: "Whether to auto-implement changes. If null, inherits from the organization.",
				Optional:    true,
				Computed:    true,
			},
			"ssh_id": schema.StringAttribute{
				Description: "ID of the SSH key to use for the workspace.",
				Optional:    true,
				Computed:    true,
			},
			"module_ssh_key": schema.StringAttribute{
				Description: "SSH key for accessing private modules.",
				Optional:    true,
				Computed:    true,
			},
			"vcs": schema.SingleNestedAttribute{
				Description: "VCS connection details associated with this workspace.",
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Description: "The VCS unique ID (UUID).",
						Computed:    true,
					},
					"name": schema.StringAttribute{
						Description: "The name of the VCS connection.",
						Computed:    true,
					},
					"vcs_type": schema.StringAttribute{
						Description: "The type of VCS (e.g., github, gitlab, bitbucket).",
						Computed:    true,
					},
					"url": schema.StringAttribute{
						Description: "The URL of the VCS instance.",
						Computed:    true,
					},
					"description": schema.StringAttribute{
						Description: "A description of the VCS connection.",
						Computed:    true,
					},
					"created_at": schema.StringAttribute{
						Description: "The timestamp when the VCS was created.",
						Computed:    true,
					},
					"updated_at": schema.StringAttribute{
						Description: "The timestamp when the VCS was last updated.",
						Computed:    true,
					},
				},
			},
		},
	}
}

func (r *WorkspaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

// Helper function to convert VCS API response to types.Object
func vcsToObject(vcs *VCSAPIResponse) types.Object {
	if vcs == nil {
		return types.ObjectNull(map[string]attr.Type{
			"id":          types.StringType,
			"name":        types.StringType,
			"vcs_type":    types.StringType,
			"url":         types.StringType,
			"description": types.StringType,
			"created_at":  types.StringType,
			"updated_at":  types.StringType,
		})
	}

	return types.ObjectValueMust(
		map[string]attr.Type{
			"id":          types.StringType,
			"name":        types.StringType,
			"vcs_type":    types.StringType,
			"url":         types.StringType,
			"description": types.StringType,
			"created_at":  types.StringType,
			"updated_at":  types.StringType,
		},
		map[string]attr.Value{
			"id":          types.StringValue(vcs.ID),
			"name":        types.StringValue(vcs.Name),
			"vcs_type":    types.StringValue(vcs.VcsType),
			"url":         types.StringValue(vcs.URL),
			"description": types.StringValue(vcs.Description),
			"created_at":  types.StringValue(vcs.CreatedAt.Format(time.RFC3339)),
			"updated_at":  types.StringValue(vcs.UpdatedAt.Format(time.RFC3339)),
		},
	)
}

func mapWorkspaceResponseToModel(ctx context.Context, data *WorkspaceResourceModel, workspace WorkspaceAPIResponse) {
	data.ID = types.StringValue(workspace.ID)
	data.Name = types.StringValue(workspace.Name)
	data.Description = types.StringValue(workspace.Description)
	data.Source = types.StringValue(workspace.Source)
	data.Branch = types.StringValue(workspace.Branch)
	data.TerraformVersion = types.StringValue(workspace.TerraformVersion)
	data.CreatedAt = types.StringValue(workspace.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(workspace.UpdatedAt.Format(time.RFC3339))
	data.VCS = vcsToObject(workspace.VCS)
	data.Locked = types.BoolValue(workspace.Locked)
	data.AutoApply = types.BoolValue(workspace.AutoApply)
	if workspace.IacType != "" {
		data.IacType = types.StringValue(workspace.IacType)
	}
	if workspace.DefaultJobAction != "" {
		data.DefaultJobAction = types.StringValue(workspace.DefaultJobAction)
	}
	if workspace.WorkerPool != nil {
		data.WorkerPoolID = types.StringValue(*workspace.WorkerPool)
	}
	if workspace.Folder != "" {
		data.Folder = types.StringValue(workspace.Folder)
	}
	if workspace.ExecutionMode != "" {
		data.ExecutionMode = types.StringValue(workspace.ExecutionMode)
	}
	data.AgentsEnabled = types.BoolValue(workspace.AgentsEnabled)
	if workspace.DriftDetectionEnabled != nil {
		data.DriftDetectionEnabled = types.BoolValue(*workspace.DriftDetectionEnabled)
	} else {
		data.DriftDetectionEnabled = types.BoolNull()
	}
	if workspace.RemedyDrift != nil {
		data.RemedyDrift = types.BoolValue(*workspace.RemedyDrift)
	} else {
		data.RemedyDrift = types.BoolNull()
	}
	if workspace.AutoImplementChanges != nil {
		data.AutoImplementChanges = types.BoolValue(*workspace.AutoImplementChanges)
	} else {
		data.AutoImplementChanges = types.BoolNull()
	}
	data.SshId = types.StringValue(workspace.SshId)
	data.ModuleSshKey = types.StringValue(workspace.ModuleSshKey)
	if workspace.Tags != nil {
		tagMap := map[string]attr.Value{}
		for k, v := range workspace.Tags {
			tagMap[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		data.Tags = types.MapValueMust(types.StringType, tagMap)
	} else {
		data.Tags = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
}

func (r *WorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := WorkspaceCreateRequest{
		Name:             data.Name.ValueString(),
		Description:      data.Description.ValueString(),
		Source:           data.Source.ValueString(),
		Branch:           data.Branch.ValueString(),
		TerraformVersion: data.TerraformVersion.ValueString(),
		AutoApply:        data.AutoApply.ValueBool(),
		IacType:          data.IacType.ValueString(),
		DefaultJobAction: data.DefaultJobAction.ValueString(),
		Folder:           data.Folder.ValueString(),
		ExecutionMode:    data.ExecutionMode.ValueString(),
		AgentsEnabled:    data.AgentsEnabled.ValueBool(),
	}
	if !data.WorkerPoolID.IsNull() && data.WorkerPoolID.ValueString() != "" {
		createReq.WorkerPool = data.WorkerPoolID.ValueString()
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
	if !data.DriftDetectionEnabled.IsNull() {
		v := data.DriftDetectionEnabled.ValueBool()
		createReq.DriftDetectionEnabled = &v
	}
	if !data.RemedyDrift.IsNull() {
		v := data.RemedyDrift.ValueBool()
		createReq.RemedyDrift = &v
	}
	if !data.AutoImplementChanges.IsNull() {
		v := data.AutoImplementChanges.ValueBool()
		createReq.AutoImplementChanges = &v
	}
	if !data.SshId.IsNull() {
		createReq.SshId = data.SshId.ValueString()
	}
	if !data.ModuleSshKey.IsNull() {
		createReq.ModuleSshKey = data.ModuleSshKey.ValueString()
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	// POST to /api/organizations/{organization_name}/workspaces/
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/",
		r.provider.host,
		data.OrganizationName.ValueString())

	reqHttp, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(reqBody)))
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

	// Read the response body
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

	// Parse the response
	var workspace WorkspaceAPIResponse
	err = json.Unmarshal(respBody, &workspace)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	mapWorkspaceResponseToModel(ctx, &data, workspace)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// GET from /api/organizations/{organization_name}/workspaces/{workspace_name}/
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.Name.ValueString())

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

	// If 404, resource no longer exists
	if httpResp.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
		return
	}

	// Read and parse the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var workspace WorkspaceAPIResponse
	err = json.Unmarshal(respBody, &workspace)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	mapWorkspaceResponseToModel(ctx, &data, workspace)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state WorkspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WorkspaceResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := WorkspaceUpdateRequest{}

	if !plan.Name.Equal(state.Name) {
		updateReq.Name = plan.Name.ValueString()
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = plan.Description.ValueString()
	}
	if !plan.Source.Equal(state.Source) {
		updateReq.Source = plan.Source.ValueString()
	}
	if !plan.Branch.Equal(state.Branch) {
		updateReq.Branch = plan.Branch.ValueString()
	}
	if !plan.TerraformVersion.Equal(state.TerraformVersion) {
		updateReq.TerraformVersion = plan.TerraformVersion.ValueString()
	}
	if !plan.AutoApply.Equal(state.AutoApply) {
		v := plan.AutoApply.ValueBool()
		updateReq.AutoApply = &v
	}
	if !plan.IacType.Equal(state.IacType) {
		updateReq.IacType = plan.IacType.ValueString()
	}
	if !plan.DefaultJobAction.Equal(state.DefaultJobAction) {
		updateReq.DefaultJobAction = plan.DefaultJobAction.ValueString()
	}
	if !plan.WorkerPoolID.Equal(state.WorkerPoolID) {
		updateReq.WorkerPool = plan.WorkerPoolID.ValueString()
	}
	if !plan.Folder.Equal(state.Folder) {
		updateReq.Folder = plan.Folder.ValueString()
	}
	if !plan.ExecutionMode.Equal(state.ExecutionMode) {
		updateReq.ExecutionMode = plan.ExecutionMode.ValueString()
	}
	if !plan.AgentsEnabled.Equal(state.AgentsEnabled) {
		v := plan.AgentsEnabled.ValueBool()
		updateReq.AgentsEnabled = &v
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
	if !plan.DriftDetectionEnabled.Equal(state.DriftDetectionEnabled) {
		if !plan.DriftDetectionEnabled.IsNull() {
			v := plan.DriftDetectionEnabled.ValueBool()
			updateReq.DriftDetectionEnabled = &v
		}
	}
	if !plan.RemedyDrift.Equal(state.RemedyDrift) {
		if !plan.RemedyDrift.IsNull() {
			v := plan.RemedyDrift.ValueBool()
			updateReq.RemedyDrift = &v
		}
	}
	if !plan.AutoImplementChanges.Equal(state.AutoImplementChanges) {
		if !plan.AutoImplementChanges.IsNull() {
			v := plan.AutoImplementChanges.ValueBool()
			updateReq.AutoImplementChanges = &v
		}
	}
	if !plan.SshId.Equal(state.SshId) && !plan.SshId.IsNull() {
		updateReq.SshId = plan.SshId.ValueString()
	}
	if !plan.ModuleSshKey.Equal(state.ModuleSshKey) && !plan.ModuleSshKey.IsNull() {
		updateReq.ModuleSshKey = plan.ModuleSshKey.ValueString()
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	// PATCH to /api/organizations/{organization_name}/workspaces/{workspace_name}/
	// Use state name (current name) for the URL, not plan name (which might be changing)
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/",
		r.provider.host,
		plan.OrganizationName.ValueString(),
		state.Name.ValueString())

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

	// Read the response body
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

	// Parse the response
	var workspace WorkspaceAPIResponse
	err = json.Unmarshal(respBody, &workspace)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	mapWorkspaceResponseToModel(ctx, &plan, workspace)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *WorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DELETE from /api/organizations/{organization_name}/workspaces/{workspace_name}/
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.Name.ValueString())

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

	// Read the response body for error details if needed
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

	// Remove resource from state
	resp.State.RemoveResource(ctx)
}

func (r *WorkspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Parse the import ID: format is "organization_name:workspace_name"
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:workspace_name'",
		)
		return
	}

	organizationName := parts[0]
	workspaceName := parts[1]

	// Use the list endpoint and filter by name (same approach as datasource)
	url := fmt.Sprintf("https://%s/api/organizations/%s/workspaces/",
		r.provider.host,
		organizationName)

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

	if httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"Failed to fetch workspaces",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	// Read and parse the response body (list of workspaces)
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var workspaces []WorkspaceAPIResponse
	err = json.Unmarshal(respBody, &workspaces)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	// Find the workspace by name
	var workspace *WorkspaceAPIResponse
	found := false
	for i := range workspaces {
		if workspaces[i].Name == workspaceName {
			workspace = &workspaces[i]
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Workspace not found",
			fmt.Sprintf("No workspace with name '%s' found in organization '%s'", workspaceName, organizationName),
		)
		return
	}

	var data WorkspaceResourceModel
	data.OrganizationName = types.StringValue(organizationName)
	mapWorkspaceResponseToModel(ctx, &data, *workspace)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

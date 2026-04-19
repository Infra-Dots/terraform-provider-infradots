package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &AgentSkillResource{}
	_ resource.ResourceWithImportState = &AgentSkillResource{}
)

func NewAgentSkillResource() resource.Resource {
	return &AgentSkillResource{}
}

// ── Terraform model ──────────────────────────────────────────────────────────

type AgentSkillResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	DisplayName      types.String `tfsdk:"display_name"`
	Description      types.String `tfsdk:"description"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	Config           types.String `tfsdk:"config"`
	SourceRepo       types.String `tfsdk:"source_repo"`
	SourcePath       types.String `tfsdk:"source_path"`
	SourceRef        types.String `tfsdk:"source_ref"`
	IsGithubSourced  types.Bool   `tfsdk:"is_github_sourced"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

// ── API response / request structs ──────────────────────────────────────────

type AgentSkillAPIResponse struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	DisplayName     string          `json:"display_name"`
	Description     string          `json:"description"`
	Enabled         bool            `json:"enabled"`
	Config          json.RawMessage `json:"config"`
	SourceRepo      string          `json:"source_repo"`
	SourcePath      string          `json:"source_path"`
	SourceRef       string          `json:"source_ref"`
	IsGithubSourced bool            `json:"is_github_sourced"`
	CreatedAt       time.Time       `json:"created_at"`
}

type AgentSkillCreateRequest struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description,omitempty"`
	Enabled     bool            `json:"enabled"`
	Config      json.RawMessage `json:"config,omitempty"`
	SourceRepo  string          `json:"source_repo,omitempty"`
	SourcePath  string          `json:"source_path,omitempty"`
	SourceRef   string          `json:"source_ref,omitempty"`
}

type AgentSkillUpdateRequest struct {
	DisplayName string          `json:"display_name,omitempty"`
	Description string          `json:"description,omitempty"`
	Enabled     *bool           `json:"enabled,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
	SourceRepo  string          `json:"source_repo,omitempty"`
	SourcePath  string          `json:"source_path,omitempty"`
	SourceRef   string          `json:"source_ref,omitempty"`
}

// ── Resource struct ──────────────────────────────────────────────────────────

type AgentSkillResource struct {
	provider *InfradotsProvider
}

func (r *AgentSkillResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_agent_skill"
}

func (r *AgentSkillResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An AI agent skill scoped to an organization in InfraDots.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The skill UUID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The organization this skill belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Slug identifier used in code (e.g. 'review', 'bootstrap'). Unique per organization.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				Description: "Human-readable name for the skill.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Optional description of what the skill does.",
				Optional:    true,
				Computed:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the skill is enabled. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"config": schema.StringAttribute{
				Description: "JSON string of runtime configuration overrides (model, temperature, system_prompt_prefix, etc.).",
				Optional:    true,
				Computed:    true,
			},
			"source_repo": schema.StringAttribute{
				Description: "GitHub repository URL for user-defined skills.",
				Optional:    true,
				Computed:    true,
			},
			"source_path": schema.StringAttribute{
				Description: "Path to the skill entrypoint within the repository.",
				Optional:    true,
				Computed:    true,
			},
			"source_ref": schema.StringAttribute{
				Description: "Branch, tag, or commit SHA to use from the repository.",
				Optional:    true,
				Computed:    true,
			},
			"is_github_sourced": schema.BoolAttribute{
				Description: "True when source_repo is set.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the skill was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AgentSkillResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.provider = req.ProviderData.(*InfradotsProvider)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func agentSkillAPIToModel(data *AgentSkillResourceModel, skill AgentSkillAPIResponse) {
	data.ID = types.StringValue(skill.ID)
	data.Name = types.StringValue(skill.Name)
	data.DisplayName = types.StringValue(skill.DisplayName)
	data.Description = types.StringValue(skill.Description)
	data.Enabled = types.BoolValue(skill.Enabled)
	data.SourceRepo = types.StringValue(skill.SourceRepo)
	data.SourcePath = types.StringValue(skill.SourcePath)
	data.SourceRef = types.StringValue(skill.SourceRef)
	data.IsGithubSourced = types.BoolValue(skill.IsGithubSourced)
	data.CreatedAt = types.StringValue(skill.CreatedAt.Format(time.RFC3339))

	// Normalise config: store as compact JSON string, or empty string if null/empty object
	if len(skill.Config) == 0 || string(skill.Config) == "null" || string(skill.Config) == "{}" {
		data.Config = types.StringValue("")
	} else {
		data.Config = types.StringValue(string(skill.Config))
	}
}

func (r *AgentSkillResource) skillURL(orgName, id string) string {
	if id == "" {
		return fmt.Sprintf("https://%s/api/organizations/%s/skills/", r.provider.host, orgName)
	}
	return fmt.Sprintf("https://%s/api/organizations/%s/skills/%s/", r.provider.host, orgName, id)
}

func configToRawMessage(configStr string) (json.RawMessage, error) {
	if configStr == "" {
		return nil, nil
	}
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(configStr), &raw); err != nil {
		return nil, fmt.Errorf("config must be valid JSON: %w", err)
	}
	return raw, nil
}

// ── Create ───────────────────────────────────────────────────────────────────

func (r *AgentSkillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentSkillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configRaw, err := configToRawMessage(data.Config.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid config JSON", err.Error())
		return
	}

	body := AgentSkillCreateRequest{
		Name:        data.Name.ValueString(),
		DisplayName: data.DisplayName.ValueString(),
		Description: data.Description.ValueString(),
		Enabled:     data.Enabled.ValueBool(),
		Config:      configRaw,
		SourceRepo:  data.SourceRepo.ValueString(),
		SourcePath:  data.SourcePath.ValueString(),
		SourceRef:   data.SourceRef.ValueString(),
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	httpReq, err := http.NewRequest(http.MethodPost, r.skillURL(data.OrganizationName.ValueString(), ""), strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response", err.Error())
		return
	}
	if httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError("Create failed", fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)))
		return
	}

	var skill AgentSkillAPIResponse
	if err := json.Unmarshal(respBody, &skill); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	agentSkillAPIToModel(&data, skill)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ── Read ─────────────────────────────────────────────────────────────────────

func (r *AgentSkillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentSkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpReq, err := http.NewRequest(http.MethodGet, r.skillURL(data.OrganizationName.ValueString(), data.ID.ValueString()), nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(httpReq)
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
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status: %d", httpResp.StatusCode))
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response", err.Error())
		return
	}

	var skill AgentSkillAPIResponse
	if err := json.Unmarshal(respBody, &skill); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	agentSkillAPIToModel(&data, skill)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ── Update ───────────────────────────────────────────────────────────────────

func (r *AgentSkillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentSkillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retain ID from state
	var state AgentSkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ID = state.ID

	configRaw, err := configToRawMessage(data.Config.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid config JSON", err.Error())
		return
	}

	enabled := data.Enabled.ValueBool()
	body := AgentSkillUpdateRequest{
		DisplayName: data.DisplayName.ValueString(),
		Description: data.Description.ValueString(),
		Enabled:     &enabled,
		Config:      configRaw,
		SourceRepo:  data.SourceRepo.ValueString(),
		SourcePath:  data.SourcePath.ValueString(),
		SourceRef:   data.SourceRef.ValueString(),
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	httpReq, err := http.NewRequest(http.MethodPatch, r.skillURL(data.OrganizationName.ValueString(), data.ID.ValueString()), strings.NewReader(string(reqBody)))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := r.provider.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response", err.Error())
		return
	}
	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("Update failed", fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)))
		return
	}

	var skill AgentSkillAPIResponse
	if err := json.Unmarshal(respBody, &skill); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	agentSkillAPIToModel(&data, skill)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ── Delete ───────────────────────────────────────────────────────────────────

func (r *AgentSkillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentSkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpReq, err := http.NewRequest(http.MethodDelete, r.skillURL(data.OrganizationName.ValueString(), data.ID.ValueString()), nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("Delete failed", fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)))
	}
}

// ── ImportState ──────────────────────────────────────────────────────────────

func (r *AgentSkillResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "organization_name:skill_id"
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:skill_id'",
		)
		return
	}

	orgName := parts[0]
	skillID := parts[1]

	httpReq, err := http.NewRequest(http.MethodGet, r.skillURL(orgName, skillID), nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == 404 {
		resp.Diagnostics.AddError("Skill not found", fmt.Sprintf("No skill with ID %q in organization %q", skillID, orgName))
		return
	}
	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status: %d", httpResp.StatusCode))
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response", err.Error())
		return
	}

	var skill AgentSkillAPIResponse
	if err := json.Unmarshal(respBody, &skill); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var data AgentSkillResourceModel
	data.OrganizationName = types.StringValue(orgName)
	agentSkillAPIToModel(&data, skill)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

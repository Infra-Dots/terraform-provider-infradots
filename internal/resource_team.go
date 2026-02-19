package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource              = &TeamResource{}
	_ resource.ResourceWithConfigure = &TeamResource{}
)

func NewTeamResource() resource.Resource {
	return &TeamResource{}
}

type TeamResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationName types.String `tfsdk:"organization_name"`
	Name             types.String `tfsdk:"name"`
	Members          types.List   `tfsdk:"members"`
}

type TeamAPIResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Members     []map[string]string `json:"members"`
	Permissions []interface{}       `json:"permissions"`
}

type TeamCreateRequest struct {
	Name    string   `json:"name"`
	Members []string `json:"members,omitempty"`
}

type TeamUpdateRequest struct {
	Name string `json:"name,omitempty"`
}

type TeamResource struct {
	provider *InfradotsProvider
}

func (r *TeamResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_team"
}

func (r *TeamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Team in an InfraDots organization",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The team unique ID (UUID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this team belongs to.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the team.",
				Required:    true,
			},
			"members": schema.ListAttribute{
				Description: "List of member email addresses in the team.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *TeamResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func teamMembersToList(members []map[string]string) types.List {
	if members == nil || len(members) == 0 {
		return types.ListValueMust(types.StringType, []attr.Value{})
	}
	memberAttrs := make([]attr.Value, 0, len(members))
	for _, m := range members {
		if email, ok := m["email"]; ok {
			memberAttrs = append(memberAttrs, types.StringValue(email))
		}
	}
	return types.ListValueMust(types.StringType, memberAttrs)
}

func (r *TeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := TeamCreateRequest{
		Name: data.Name.ValueString(),
	}

	if !data.Members.IsNull() && !data.Members.IsUnknown() {
		var members []string
		diags = data.Members.ElementsAs(ctx, &members, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Members = members
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/teams/",
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

	var team TeamAPIResponse
	err = json.Unmarshal(respBody, &team)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(team.ID)
	data.Name = types.StringValue(team.Name)
	data.Members = teamMembersToList(team.Members)

	diags = resp.State.Set(ctx, &data)
	tflog.Info(ctx, "Team Resource Created", map[string]any{"success": true})
	resp.Diagnostics.Append(diags...)
}

func (r *TeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/teams/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.ID.ValueString())

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

	var team TeamAPIResponse
	err = json.Unmarshal(respBody, &team)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(team.ID)
	data.Name = types.StringValue(team.Name)
	data.Members = teamMembersToList(team.Members)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *TeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state TeamResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan TeamResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update team name if changed
	if !plan.Name.Equal(state.Name) {
		updateReq := TeamUpdateRequest{
			Name: plan.Name.ValueString(),
		}

		reqBody, err := json.Marshal(updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error marshaling request", err.Error())
			return
		}

		url := fmt.Sprintf("https://%s/api/organizations/%s/teams/%s/",
			r.provider.host,
			plan.OrganizationName.ValueString(),
			state.ID.ValueString())

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

		if httpResp.StatusCode != 200 {
			respBody, _ := io.ReadAll(httpResp.Body)
			resp.Diagnostics.AddError(
				"Update failed",
				fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
			)
			return
		}
	}

	// Update members if changed
	if !plan.Members.Equal(state.Members) && !plan.Members.IsNull() && !plan.Members.IsUnknown() {
		var members []string
		diags = plan.Members.ElementsAs(ctx, &members, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		membersReq := map[string][]string{"members": members}
		reqBody, err := json.Marshal(membersReq)
		if err != nil {
			resp.Diagnostics.AddError("Error marshaling members request", err.Error())
			return
		}

		url := fmt.Sprintf("https://%s/api/organizations/%s/teams/%s/members/",
			r.provider.host,
			plan.OrganizationName.ValueString(),
			state.ID.ValueString())

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

		if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
			respBody, _ := io.ReadAll(httpResp.Body)
			resp.Diagnostics.AddError(
				"Update members failed",
				fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
			)
			return
		}
	}

	// Read back the team to get updated state
	plan.ID = state.ID

	readUrl := fmt.Sprintf("https://%s/api/organizations/%s/teams/%s/",
		r.provider.host,
		plan.OrganizationName.ValueString(),
		plan.ID.ValueString())

	reqHttp, err := http.NewRequest(http.MethodGet, readUrl, nil)
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

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Read after update failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	var team TeamAPIResponse
	err = json.Unmarshal(respBody, &team)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	plan.ID = types.StringValue(team.ID)
	plan.Name = types.StringValue(team.Name)
	plan.Members = teamMembersToList(team.Members)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *TeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/organizations/%s/teams/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.ID.ValueString())

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

func (r *TeamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:team_name'",
		)
		return
	}

	organizationName := parts[0]
	teamName := parts[1]

	url := fmt.Sprintf("https://%s/api/organizations/%s/teams/",
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
			"Failed to fetch teams",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var teams []TeamAPIResponse
	err = json.Unmarshal(respBody, &teams)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var found *TeamAPIResponse
	for i := range teams {
		if teams[i].Name == teamName {
			found = &teams[i]
			break
		}
	}

	if found == nil {
		resp.Diagnostics.AddError(
			"Team not found",
			fmt.Sprintf("No team with name '%s' found in organization '%s'", teamName, organizationName),
		)
		return
	}

	var data TeamResourceModel
	data.ID = types.StringValue(found.ID)
	data.OrganizationName = types.StringValue(organizationName)
	data.Name = types.StringValue(found.Name)
	data.Members = teamMembersToList(found.Members)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

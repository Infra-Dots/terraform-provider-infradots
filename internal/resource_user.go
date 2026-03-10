package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure we fully satisfy the resource.Resource interface.
var _ resource.Resource = &UserResource{}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

// UserResourceModel maps the user resource schema data.
type UserResourceModel struct {
	ID               types.String `tfsdk:"id"`                // User UUID
	OrganizationName types.String `tfsdk:"organization_name"` // Name of the organization
	Email            types.String `tfsdk:"email"`             // User email address
	LastLogin        types.String `tfsdk:"last_login"`        // Last login timestamp
	Teams            types.List   `tfsdk:"teams"`             // List of teams
	Permissions      types.List   `tfsdk:"permissions"`       // List of permissions
}

// UserAPIResponse represents the JSON structure returned by the API
type UserAPIResponse struct {
	ID          string                 `json:"id"`
	Email       string                 `json:"email"`
	LastLogin   *time.Time             `json:"last_login"`
	Teams       []interface{}          `json:"teams"`
	Permissions []UserPermissionObject `json:"permissions"`
}

// UserPermissionObject represents a permission in the API response
type UserPermissionObject struct {
	User         string `json:"user"`
	Permission   string `json:"permission"`
	Organization string `json:"organization,omitempty"`
	Workspace    string `json:"workspace,omitempty"`
}

// UserCreateRequest represents the JSON structure for creating a user
type UserCreateRequest struct {
	Members []string `json:"members"`
}

type UserResource struct {
	provider *InfradotsProvider
}

func (r *UserResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_user"
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The user unique ID (UUID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_name": schema.StringAttribute{
				Description: "The name of the organization this user belongs to.",
				Required:    true,
			},
			"email": schema.StringAttribute{
				Description: "The email address of the user.",
				Required:    true,
			},
			"last_login": schema.StringAttribute{
				Description: "The timestamp when the user last logged in.",
				Computed:    true,
			},
			"teams": schema.ListAttribute{
				Description: "List of teams the user belongs to.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"permissions": schema.ListAttribute{
				Description: "List of permissions assigned to the user.",
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"user":         types.StringType,
						"permission":   types.StringType,
						"organization": types.StringType,
						"workspace":    types.StringType,
					},
				},
				Computed: true,
			},
		},
	}
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare the request - API expects an array of member emails
	createReq := UserCreateRequest{
		Members: []string{data.Email.ValueString()},
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	// POST to /api/users/{organization_name}/users/
	url := fmt.Sprintf("https://%s/api/users/%s/users/",
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

	// API returns 200 on success, not 201
	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Non-200 response",
			fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	// After creating, we need to read the user to get the full details
	// The create endpoint returns {"message": "Users added to organization successfully"}
	// So we need to list users and find the one we just created
	readDiags := r.readUser(ctx, &data)
	resp.Diagnostics.Append(readDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data back into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	readDiags := r.readUser(ctx, &data)
	resp.Diagnostics.Append(readDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If user not found, remove from state
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save (possibly updated) state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// readUser is a helper function that fetches user data from the API
func (r *UserResource) readUser(ctx context.Context, data *UserResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// GET from /api/users/{organization_name}/users/
	url := fmt.Sprintf("https://%s/api/users/%s/users/",
		r.provider.host,
		data.OrganizationName.ValueString())

	reqHttp, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return append(diags, diag.NewErrorDiagnostic(
			"Error creating request",
			err.Error(),
		))
	}
	reqHttp.Header.Set("Authorization", "Bearer "+r.provider.token)

	httpResp, err := r.provider.client.Do(reqHttp)
	if err != nil {
		return append(diags, diag.NewErrorDiagnostic(
			"HTTP request failed",
			err.Error(),
		))
	}
	defer httpResp.Body.Close()

	// If 404, resource no longer exists
	if httpResp.StatusCode == 404 {
		return append(diags, diag.NewErrorDiagnostic(
			"Resource not found",
			"Organization or user not found",
		))
	}

	if httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		return append(diags, diag.NewErrorDiagnostic(
			"Read failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		))
	}

	// Read and parse the response body (array of users)
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return append(diags, diag.NewErrorDiagnostic(
			"Error reading response body",
			err.Error(),
		))
	}

	var users []UserAPIResponse
	err = json.Unmarshal(respBody, &users)
	if err != nil {
		return append(diags, diag.NewErrorDiagnostic(
			"Error parsing response",
			err.Error(),
		))
	}

	// Find the user with matching email
	var foundUser *UserAPIResponse
	for i := range users {
		if users[i].Email == data.Email.ValueString() {
			foundUser = &users[i]
			break
		}
	}

	if foundUser == nil {
		// User not found - clear the ID to signal removal
		data.ID = types.StringNull()
		return nil
	}

	// Update the model with the response data
	data.ID = types.StringValue(foundUser.ID)
	data.Email = types.StringValue(foundUser.Email)

	// Handle last_login (can be null)
	if foundUser.LastLogin != nil {
		data.LastLogin = types.StringValue(foundUser.LastLogin.Format(time.RFC3339))
	} else {
		data.LastLogin = types.StringNull()
	}

	// Convert teams to list
	if foundUser.Teams != nil && len(foundUser.Teams) > 0 {
		teamAttrs := make([]attr.Value, len(foundUser.Teams))
		for i, team := range foundUser.Teams {
			// Teams might be strings or objects, handle both
			var teamStr string
			if teamStrVal, ok := team.(string); ok {
				teamStr = teamStrVal
			} else if teamMap, ok := team.(map[string]interface{}); ok {
				// If it's an object, try to get a name or id field
				if name, ok := teamMap["name"].(string); ok {
					teamStr = name
				} else if id, ok := teamMap["id"].(string); ok {
					teamStr = id
				} else {
					teamStr = fmt.Sprintf("%v", team)
				}
			} else {
				teamStr = fmt.Sprintf("%v", team)
			}
			teamAttrs[i] = types.StringValue(teamStr)
		}
		data.Teams = types.ListValueMust(types.StringType, teamAttrs)
	} else {
		data.Teams = types.ListValueMust(types.StringType, []attr.Value{})
	}

	// Convert permissions to list of objects
	if foundUser.Permissions != nil {
		permissionAttrs := make([]attr.Value, len(foundUser.Permissions))
		for i, perm := range foundUser.Permissions {
			permMap := map[string]attr.Value{
				"user":         types.StringValue(perm.User),
				"permission":   types.StringValue(perm.Permission),
				"organization": types.StringValue(perm.Organization),
				"workspace":    types.StringValue(perm.Workspace),
			}
			permissionAttrs[i] = types.ObjectValueMust(
				map[string]attr.Type{
					"user":         types.StringType,
					"permission":   types.StringType,
					"organization": types.StringType,
					"workspace":    types.StringType,
				},
				permMap,
			)
		}
		data.Permissions = types.ListValueMust(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"user":         types.StringType,
					"permission":   types.StringType,
					"organization": types.StringType,
					"workspace":    types.StringType,
				},
			},
			permissionAttrs,
		)
	} else {
		data.Permissions = types.ListNull(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"user":         types.StringType,
					"permission":   types.StringType,
					"organization": types.StringType,
					"workspace":    types.StringType,
				},
			},
		)
	}

	return nil
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// For users, we don't support updates through this resource
	// The email and organization_name are the identifying fields
	// If they change, Terraform will destroy and recreate the resource
	var plan UserResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Just read the current state to ensure it's up to date
	readDiags := r.readUser(ctx, &plan)
	resp.Diagnostics.Append(readDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated info
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DELETE from /api/users/{organization_name}/users/{member_email}/
	url := fmt.Sprintf("https://%s/api/users/%s/users/%s/",
		r.provider.host,
		data.OrganizationName.ValueString(),
		data.Email.ValueString())

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

	// API returns 204 on successful deletion
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

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Parse the import ID: format is "organization_name:email"
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'organization_name:email'",
		)
		return
	}

	organizationName := parts[0]
	email := parts[1]

	// Create the state model
	var data UserResourceModel
	data.OrganizationName = types.StringValue(organizationName)
	data.Email = types.StringValue(email)

	readDiags := r.readUser(ctx, &data)
	resp.Diagnostics.Append(readDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set the state
	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

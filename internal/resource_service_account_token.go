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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ServiceAccountTokenResource{}
	_ resource.ResourceWithImportState = &ServiceAccountTokenResource{}
)

func NewServiceAccountTokenResource() resource.Resource {
	return &ServiceAccountTokenResource{}
}

type ServiceAccountTokenResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	Description      types.String `tfsdk:"description"`
	Expiration       types.String `tfsdk:"expiration"`
	CreatedAt        types.String `tfsdk:"created_at"`
	LastUsed         types.String `tfsdk:"last_used"`
	JWT              types.String `tfsdk:"jwt"`
}

type ServiceAccountTokenObject struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Expiration  *time.Time `json:"expiration"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsed    *time.Time `json:"last_used"`
}

type ServiceAccountTokenCreateResponse struct {
	Token ServiceAccountTokenObject `json:"token"`
	JWT   string                    `json:"jwt"`
}

type ServiceAccountTokenCreateRequest struct {
	Description string `json:"description"`
	Expiration  string `json:"expiration,omitempty"`
}

type ServiceAccountTokenResource struct {
	provider *InfradotsProvider
}

func (r *ServiceAccountTokenResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "infradots_service_account_token"
}

func (r *ServiceAccountTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Service Account Token in InfraDots Platform (admin-only).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The token unique ID (UUID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_account_id": schema.StringAttribute{
				Description: "The ID of the service account this token belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Description of the token.",
				Required:    true,
			},
			"expiration": schema.StringAttribute{
				Description: "Expiration date of the token (RFC3339). If omitted, the token does not expire.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the token was created.",
				Computed:    true,
			},
			"last_used": schema.StringAttribute{
				Description: "The timestamp when the token was last used.",
				Computed:    true,
			},
			"jwt": schema.StringAttribute{
				Description: "The JWT value of the token. Only available at creation time.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ServiceAccountTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		if provider, ok := req.ProviderData.(*InfradotsProvider); ok {
			r.provider = provider
		}
	}
}

func (r *ServiceAccountTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceAccountTokenResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := ServiceAccountTokenCreateRequest{
		Description: data.Description.ValueString(),
	}
	if !data.Expiration.IsNull() && !data.Expiration.IsUnknown() {
		createReq.Expiration = data.Expiration.ValueString()
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request", err.Error())
		return
	}

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/%s/tokens/",
		r.provider.host,
		data.ServiceAccountID.ValueString())
	httpReq, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(reqBody)))
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

	var createResp ServiceAccountTokenCreateResponse
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	tok := createResp.Token
	data.ID = types.StringValue(tok.ID)
	data.Description = types.StringValue(tok.Description)
	data.CreatedAt = types.StringValue(tok.CreatedAt.Format(time.RFC3339))
	if tok.Expiration != nil {
		data.Expiration = types.StringValue(tok.Expiration.Format(time.RFC3339))
	} else {
		data.Expiration = types.StringNull()
	}
	if tok.LastUsed != nil {
		data.LastUsed = types.StringValue(tok.LastUsed.Format(time.RFC3339))
	} else {
		data.LastUsed = types.StringValue("")
	}
	data.JWT = types.StringValue(createResp.JWT)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceAccountTokenResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/%s/tokens/%s/",
		r.provider.host,
		data.ServiceAccountID.ValueString(),
		data.ID.ValueString())
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
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
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var tok ServiceAccountTokenObject
	if err := json.Unmarshal(respBody, &tok); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	data.ID = types.StringValue(tok.ID)
	data.Description = types.StringValue(tok.Description)
	data.CreatedAt = types.StringValue(tok.CreatedAt.Format(time.RFC3339))
	if tok.Expiration != nil {
		data.Expiration = types.StringValue(tok.Expiration.Format(time.RFC3339))
	} else {
		data.Expiration = types.StringNull()
	}
	if tok.LastUsed != nil {
		data.LastUsed = types.StringValue(tok.LastUsed.Format(time.RFC3339))
	} else {
		data.LastUsed = types.StringValue("")
	}
	// jwt is write-once: never update from API on Read, keep existing state value.

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountTokenResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All mutable attributes have RequiresReplace, so Update is never called.
	resp.Diagnostics.AddError("Update not supported", "Service account tokens cannot be updated in-place.")
}

func (r *ServiceAccountTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceAccountTokenResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/%s/tokens/%s/",
		r.provider.host,
		data.ServiceAccountID.ValueString(),
		data.ID.ValueString())
	httpReq, err := http.NewRequest(http.MethodDelete, url, nil)
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

func (r *ServiceAccountTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: "sa_id:token_id"
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Import ID must be in the format 'service_account_id:token_id'",
		)
		return
	}
	saID := parts[0]
	tokenID := parts[1]

	url := fmt.Sprintf("https://%s/api/admin/service-accounts/%s/tokens/%s/",
		r.provider.host, saID, tokenID)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
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

	if httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"Import failed",
			fmt.Sprintf("Status code: %d, Body: %s", httpResp.StatusCode, string(respBody)),
		)
		return
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading response body", err.Error())
		return
	}

	var tok ServiceAccountTokenObject
	if err := json.Unmarshal(respBody, &tok); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var data ServiceAccountTokenResourceModel
	data.ID = types.StringValue(tok.ID)
	data.ServiceAccountID = types.StringValue(saID)
	data.Description = types.StringValue(tok.Description)
	data.CreatedAt = types.StringValue(tok.CreatedAt.Format(time.RFC3339))
	if tok.Expiration != nil {
		data.Expiration = types.StringValue(tok.Expiration.Format(time.RFC3339))
	} else {
		data.Expiration = types.StringNull()
	}
	if tok.LastUsed != nil {
		data.LastUsed = types.StringValue(tok.LastUsed.Format(time.RFC3339))
	} else {
		data.LastUsed = types.StringValue("")
	}
	// jwt cannot be recovered after initial creation.
	data.JWT = types.StringValue("")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

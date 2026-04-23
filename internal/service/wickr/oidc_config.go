// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_wickr_oidc_config", name="OIDC Config")
// @IdentityAttribute("network_id")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/wickr;wickr.GetOidcInfoOutput")
// @Testing(preCheck="testAccPreCheck")
// @Testing(serialize=true)
// @Testing(tagsTest=false)
// @Testing(hasNoPreExistingResource=true)
func newOIDCConfigResource(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &oidcConfigResource{}

	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultReadTimeout(10 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(30 * time.Minute)

	return r, nil
}

const (
	ResNameOIDCConfig = "OIDC Config"
)

type oidcConfigResource struct {
	framework.ResourceWithModel[oidcConfigResourceModel]
	framework.WithTimeouts
	framework.WithImportByIdentity
}

func (r *oidcConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"network_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			// TODO: company_id region-prefix validation deferred (open question #9).
			// The company_id value requires a Wickr-specific region prefix
			// (e.g., UE1- for us-east-1, AS1- for ap-southeast-1). Verifying
			// the full prefix mapping requires live API calls in multiple
			// regions which cannot be done here. Add a regexache-compiled
			// stringvalidator.RegexMatches once the mapping is fully verified.
			"company_id": schema.StringAttribute{
				Required: true,
			},
			names.AttrIssuer: schema.StringAttribute{
				Required: true,
			},
			"scopes": schema.StringAttribute{
				Required: true,
			},
			"custom_username": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"extra_auth_params": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"secret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
			"sso_token_buffer_minutes": schema.Int32Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				Optional: true,
			},
			// validate_before_save is a provider-only flag. When true,
			// Create and Update call RegisterOidcConfigTest before
			// RegisterOidcConfig. Default false. Not sent to the API.
			"validate_before_save": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			// Computed-only attributes populated by GetOidcInfo.
			names.AttrApplicationID: schema.Int32Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.UseStateForUnknown(),
				},
			},
			"application_name": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ca_certificate": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			names.AttrClientID: schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			names.AttrClientSecret: schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"redirect_url": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			names.AttrTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *oidcConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan oidcConfigResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := plan.NetworkID.ValueString()

	// Optional pre-validation: call RegisterOidcConfigTest when
	// validate_before_save is true.
	if plan.ValidateBeforeSave.ValueBool() {
		testInput := wickr.RegisterOidcConfigTestInput{
			Issuer:    plan.Issuer.ValueStringPointer(),
			NetworkId: aws.String(networkID),
			Scopes:    plan.Scopes.ValueStringPointer(),
		}
		if !plan.ExtraAuthParams.IsNull() && !plan.ExtraAuthParams.IsUnknown() {
			testInput.ExtraAuthParams = plan.ExtraAuthParams.ValueStringPointer()
		}
		_, err := conn.RegisterOidcConfigTest(ctx, &testInput)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics,
				fmt.Errorf("OIDC pre-validation failed: %w", err),
				smerr.ID, networkID)
			return
		}
	}

	// RegisterOidcConfig is idempotent create-or-update.
	input := wickr.RegisterOidcConfigInput{
		CompanyId: plan.CompanyId.ValueStringPointer(),
		Issuer:    plan.Issuer.ValueStringPointer(),
		NetworkId: aws.String(networkID),
		Scopes:    plan.Scopes.ValueStringPointer(),
	}
	if !plan.CustomUsername.IsNull() && !plan.CustomUsername.IsUnknown() {
		input.CustomUsername = plan.CustomUsername.ValueStringPointer()
	}
	if !plan.ExtraAuthParams.IsNull() && !plan.ExtraAuthParams.IsUnknown() {
		input.ExtraAuthParams = plan.ExtraAuthParams.ValueStringPointer()
	}
	if !plan.Secret.IsNull() && !plan.Secret.IsUnknown() {
		input.Secret = plan.Secret.ValueStringPointer()
	}
	if !plan.SsoTokenBufferMinutes.IsNull() && !plan.SsoTokenBufferMinutes.IsUnknown() {
		val := plan.SsoTokenBufferMinutes.ValueInt32()
		input.SsoTokenBufferMinutes = &val
	}
	if !plan.UserId.IsNull() && !plan.UserId.IsUnknown() {
		input.UserId = plan.UserId.ValueStringPointer()
	}

	_, err := conn.RegisterOidcConfig(ctx, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Re-read to populate all Computed fields.
	out, err := findOIDCConfigByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Preserve write-only / provider-only fields before flatten.
	secret := plan.Secret
	validateBeforeSave := plan.ValidateBeforeSave

	flattenOIDCConfig(ctx, out, &plan)

	plan.Secret = secret
	plan.ValidateBeforeSave = validateBeforeSave

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *oidcConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oidcConfigResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := state.NetworkID.ValueString()

	out, err := findOIDCConfigByID(ctx, conn, networkID)
	if retry.NotFound(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	// Mirror the Network/SecurityGroup/Bot pattern: treat ForbiddenError
	// and orphaned-child deserialization failures as "gone" so terraform
	// refresh after an out-of-band parent-network delete cleanly removes
	// the resource from state.
	if isOIDCConfigOrphanedChildError(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Preserve write-only / provider-only fields from prior state.
	// On import, prior state is empty, so these will be null — that's
	// correct because the user must supply them in config.
	secret := state.Secret
	validateBeforeSave := state.ValidateBeforeSave

	flattenOIDCConfig(ctx, out, &state)

	state.Secret = secret
	state.ValidateBeforeSave = validateBeforeSave

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &state))
}

func (r *oidcConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan oidcConfigResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := plan.NetworkID.ValueString()

	// Optional pre-validation.
	if plan.ValidateBeforeSave.ValueBool() {
		testInput := wickr.RegisterOidcConfigTestInput{
			Issuer:    plan.Issuer.ValueStringPointer(),
			NetworkId: aws.String(networkID),
			Scopes:    plan.Scopes.ValueStringPointer(),
		}
		if !plan.ExtraAuthParams.IsNull() && !plan.ExtraAuthParams.IsUnknown() {
			testInput.ExtraAuthParams = plan.ExtraAuthParams.ValueStringPointer()
		}
		_, err := conn.RegisterOidcConfigTest(ctx, &testInput)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics,
				fmt.Errorf("OIDC pre-validation failed: %w", err),
				smerr.ID, networkID)
			return
		}
	}

	// RegisterOidcConfig is idempotent create-or-update.
	input := wickr.RegisterOidcConfigInput{
		CompanyId: plan.CompanyId.ValueStringPointer(),
		Issuer:    plan.Issuer.ValueStringPointer(),
		NetworkId: aws.String(networkID),
		Scopes:    plan.Scopes.ValueStringPointer(),
	}
	if !plan.CustomUsername.IsNull() && !plan.CustomUsername.IsUnknown() {
		input.CustomUsername = plan.CustomUsername.ValueStringPointer()
	}
	if !plan.ExtraAuthParams.IsNull() && !plan.ExtraAuthParams.IsUnknown() {
		input.ExtraAuthParams = plan.ExtraAuthParams.ValueStringPointer()
	}
	if !plan.Secret.IsNull() && !plan.Secret.IsUnknown() {
		input.Secret = plan.Secret.ValueStringPointer()
	}
	if !plan.SsoTokenBufferMinutes.IsNull() && !plan.SsoTokenBufferMinutes.IsUnknown() {
		val := plan.SsoTokenBufferMinutes.ValueInt32()
		input.SsoTokenBufferMinutes = &val
	}
	if !plan.UserId.IsNull() && !plan.UserId.IsUnknown() {
		input.UserId = plan.UserId.ValueStringPointer()
	}

	_, err := conn.RegisterOidcConfig(ctx, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Re-read to populate all Computed fields.
	out, err := findOIDCConfigByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Preserve write-only / provider-only fields.
	secret := plan.Secret
	validateBeforeSave := plan.ValidateBeforeSave

	flattenOIDCConfig(ctx, out, &plan)

	plan.Secret = secret
	plan.ValidateBeforeSave = validateBeforeSave

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *oidcConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op. The Wickr API does not support deleting OIDC configuration.
	// Removing this resource only drops it from Terraform state; the
	// underlying configuration remains active on the network.
}

func (r *oidcConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("network_id"), req, resp)
}

// isOIDCConfigOrphanedChildError returns true when err indicates the
// OIDC config (or its parent network) is gone. Mirrors
// isBotOrphanedChildError.
func isOIDCConfigOrphanedChildError(err error) bool {
	if err == nil {
		return false
	}
	if errs.IsA[*awstypes.ForbiddenError](err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "StatusCode: 401") ||
		strings.Contains(msg, "deserialization failed")
}

// flattenOIDCConfig populates the model from a GetOidcInfoOutput.
// GetOidcInfo wraps the config in OpenidConnectInfo — AutoFlex won't
// traverse this nesting automatically, so we flatten from the nested
// OidcConfigInfo struct. Fields that the API does not return (secret,
// validate_before_save) must be preserved by the caller.
// nosemgrep:ci.semgrep.framework.manual-flattener-functions
func flattenOIDCConfig(ctx context.Context, out *wickr.GetOidcInfoOutput, m *oidcConfigResourceModel) {
	if out == nil || out.OpenidConnectInfo == nil {
		return
	}

	info := out.OpenidConnectInfo

	m.CompanyId = fwflex.StringToFramework(ctx, info.CompanyId)
	m.Issuer = fwflex.StringToFramework(ctx, info.Issuer)
	m.Scopes = fwflex.StringToFramework(ctx, info.Scopes)
	m.CustomUsername = fwflex.StringToFramework(ctx, info.CustomUsername)
	m.ExtraAuthParams = fwflex.StringToFramework(ctx, info.ExtraAuthParams)
	m.UserId = fwflex.StringToFramework(ctx, info.UserId)
	m.ApplicationName = fwflex.StringToFramework(ctx, info.ApplicationName)
	m.CaCertificate = fwflex.StringToFramework(ctx, info.CaCertificate)
	m.ClientId = fwflex.StringToFramework(ctx, info.ClientId)
	m.ClientSecret = fwflex.StringToFramework(ctx, info.ClientSecret)
	m.RedirectUrl = fwflex.StringToFramework(ctx, info.RedirectUrl)

	if info.ApplicationId != nil {
		m.ApplicationId = types.Int32Value(*info.ApplicationId)
	} else {
		m.ApplicationId = types.Int32Null()
	}
	if info.SsoTokenBufferMinutes != nil {
		m.SsoTokenBufferMinutes = types.Int32Value(*info.SsoTokenBufferMinutes)
	} else {
		m.SsoTokenBufferMinutes = types.Int32Null()
	}
}

// oidcConfigResourceModel mirrors the Framework-resource schema.
type oidcConfigResourceModel struct {
	framework.WithRegionModel
	NetworkID             types.String   `tfsdk:"network_id"`
	CompanyId             types.String   `tfsdk:"company_id"`
	Issuer                types.String   `tfsdk:"issuer"`
	Scopes                types.String   `tfsdk:"scopes"`
	CustomUsername        types.String   `tfsdk:"custom_username"`
	ExtraAuthParams       types.String   `tfsdk:"extra_auth_params"`
	Secret                types.String   `tfsdk:"secret"`
	SsoTokenBufferMinutes types.Int32    `tfsdk:"sso_token_buffer_minutes"`
	UserId                types.String   `tfsdk:"user_id"`
	ValidateBeforeSave    types.Bool     `tfsdk:"validate_before_save"`
	ApplicationId         types.Int32    `tfsdk:"application_id"`
	ApplicationName       types.String   `tfsdk:"application_name"`
	CaCertificate         types.String   `tfsdk:"ca_certificate"`
	ClientId              types.String   `tfsdk:"client_id"`
	ClientSecret          types.String   `tfsdk:"client_secret"`
	RedirectUrl           types.String   `tfsdk:"redirect_url"`
	Timeouts              timeouts.Value `tfsdk:"timeouts"`
}

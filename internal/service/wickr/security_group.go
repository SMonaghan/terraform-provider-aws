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
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	intflex "github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	inttypes "github.com/hashicorp/terraform-provider-aws/internal/types"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_wickr_security_group", name="Security Group")
// @IdentityAttribute("network_id")
// @IdentityAttribute("security_group_id")
// @ImportIDHandler("securityGroupImportID")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/wickr/types;awstypes;awstypes.SecurityGroup")
// @Testing(importStateIdAttributes="network_id;security_group_id", importStateIdAttributesSep="flex.ResourceIdSeparator")
// @Testing(preCheck="testAccPreCheck")
// @Testing(serialize=true)
// @Testing(tagsTest=false)
// @Testing(hasNoPreExistingResource=true)
func newSecurityGroupResource(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &securityGroupResource{}

	// Default timeout matrix per design.md → "Retries and timeouts":
	// Create 30m, Read 10m, Update 30m, Delete 30m.
	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultReadTimeout(10 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(30 * time.Minute)

	return r, nil
}

const (
	ResNameSecurityGroup = "Security Group"
)

type securityGroupResource struct {
	framework.ResourceWithModel[securityGroupResourceModel]
	framework.WithTimeouts
	framework.WithImportByIdentity
}

// ValidateConfig enforces cross-field invariants that AWS Wickr
// checks server-side but that we can surface at plan time:
//
//   - `enable_restricted_global_federation = true` requires
//     `global_federation = true`. AWS returns
//     "RestrictedGlobalFederation is not supported when
//     globalFederation is disabled" (HTTP 400) otherwise. Catching it
//     at plan time avoids letting Apply fail with a less-specific
//     inconsistency-check diagnostic.
func (r *securityGroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config securityGroupResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &config))
	if resp.Diagnostics.HasError() {
		return
	}
	if config.Settings.IsNull() || config.Settings.IsUnknown() {
		return
	}
	m, d := config.Settings.ToPtr(ctx)
	if d.HasError() || m == nil {
		return
	}
	if m.EnableRestrictedGlobalFederation.ValueBool() && !m.GlobalFederation.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("settings").AtListIndex(0).AtName("enable_restricted_global_federation"),
			"Invalid AWS Wickr security group settings combination",
			"`enable_restricted_global_federation = true` requires `global_federation = true`. "+
				"Restricted federation is the subset of global federation that allow-lists "+
				"specific external networks; it cannot be enabled when global federation is "+
				"disabled. Either set `global_federation = true`, or remove "+
				"`enable_restricted_global_federation` from the `settings` block.",
		)
	}
}

// ModifyPlan rewrites null scalar leaves inside a user-provided `settings`
// block to Unknown on Create, so AWS's server-side default values can
// land in state after the apply without tripping Terraform's per-leaf
// "was null, now X" consistency check.
//
// Terraform explicitly forbids synthesizing a `settings` block when the
// user's config has none (that manifests as "block count in plan (1)
// disagrees with count in config (0)"). So this function only runs when
// the user wrote at least an empty `settings {}` block. The
// no-settings-at-all case is handled on the flatten side, where
// `flattenSecurityGroup` leaves the model's settings null when the prior
// state/plan had it null.
//
// Update and destroy plans are left untouched; plan = state is the
// stable-defaults case and Terraform will not raise drift in that
// direction.
func (r *securityGroupResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip destroy plans.
	if req.Plan.Raw.IsNull() {
		return
	}
	// Skip updates — only rewrite on Create (state is null).
	if !req.State.Raw.IsNull() {
		return
	}

	var plan securityGroupResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	// User omitted the `settings` block entirely: cannot rewrite here
	// (Terraform rejects synthesizing a block). The flatten layer
	// will keep state's settings null so the post-apply block count
	// matches plan's block count (both 0).
	if plan.Settings.IsNull() || plan.Settings.IsUnknown() {
		return
	}

	// User wrote a settings block. Rewrite each null scalar leaf
	// inside it to Unknown so AWS can populate server-side defaults
	// without tripping Terraform's per-leaf consistency check. Nested
	// sub-blocks (`calling`, `password_requirements`, `shredder`,
	// the two permitted-network lists) are left as the user wrote
	// them; the flatten layer handles the "user omitted sub-block,
	// AWS returned one" case by checking the prior state.
	m, d := plan.Settings.ToPtr(ctx)
	smerr.AddEnrich(ctx, &resp.Diagnostics, d)
	if resp.Diagnostics.HasError() || m == nil {
		return
	}

	mutated := fillNullLeavesWithUnknown(ctx, m)
	if !mutated {
		return
	}

	newSettings, d := fwtypes.NewListNestedObjectValueOfPtr(ctx, m)
	smerr.AddEnrich(ctx, &resp.Diagnostics, d)
	if resp.Diagnostics.HasError() {
		return
	}
	smerr.AddEnrich(ctx, &resp.Diagnostics,
		resp.Plan.SetAttribute(ctx, path.Root("settings"), newSettings))
}

// fillNullLeavesWithUnknown mutates `m` in-place, replacing every null
// scalar leaf with Unknown. Returns true if any leaf was changed.
// Nested sub-blocks are not touched — the flatten layer respects
// them as the user wrote them.
func fillNullLeavesWithUnknown(ctx context.Context, m *securityGroupSettingsModel) bool {
	mutated := false
	if m.AlwaysReauthenticate.IsNull() {
		m.AlwaysReauthenticate = types.BoolUnknown()
		mutated = true
	}
	if m.CheckForUpdates.IsNull() {
		m.CheckForUpdates = types.BoolUnknown()
		mutated = true
	}
	if m.EnableAtak.IsNull() {
		m.EnableAtak = types.BoolUnknown()
		mutated = true
	}
	if m.EnableCrashReports.IsNull() {
		m.EnableCrashReports = types.BoolUnknown()
		mutated = true
	}
	if m.EnableFileDownload.IsNull() {
		m.EnableFileDownload = types.BoolUnknown()
		mutated = true
	}
	if m.EnableGuestFederation.IsNull() {
		m.EnableGuestFederation = types.BoolUnknown()
		mutated = true
	}
	if m.EnableNotificationPreview.IsNull() {
		m.EnableNotificationPreview = types.BoolUnknown()
		mutated = true
	}
	if m.EnableOpenAccessOption.IsNull() {
		m.EnableOpenAccessOption = types.BoolUnknown()
		mutated = true
	}
	if m.EnableRestrictedGlobalFederation.IsNull() {
		m.EnableRestrictedGlobalFederation = types.BoolUnknown()
		mutated = true
	}
	if m.FederationMode.IsNull() {
		m.FederationMode = types.Int64Unknown()
		mutated = true
	}
	if m.FilesEnabled.IsNull() {
		m.FilesEnabled = types.BoolUnknown()
		mutated = true
	}
	if m.ForceDeviceLockout.IsNull() {
		m.ForceDeviceLockout = types.Int64Unknown()
		mutated = true
	}
	if m.ForceOpenAccess.IsNull() {
		m.ForceOpenAccess = types.BoolUnknown()
		mutated = true
	}
	if m.ForceReadReceipts.IsNull() {
		m.ForceReadReceipts = types.BoolUnknown()
		mutated = true
	}
	if m.GlobalFederation.IsNull() {
		m.GlobalFederation = types.BoolUnknown()
		mutated = true
	}
	if m.IsAtoEnabled.IsNull() {
		m.IsAtoEnabled = types.BoolUnknown()
		mutated = true
	}
	if m.IsLinkPreviewEnabled.IsNull() {
		m.IsLinkPreviewEnabled = types.BoolUnknown()
		mutated = true
	}
	if m.LocationAllowMaps.IsNull() {
		m.LocationAllowMaps = types.BoolUnknown()
		mutated = true
	}
	if m.LocationEnabled.IsNull() {
		m.LocationEnabled = types.BoolUnknown()
		mutated = true
	}
	if m.LockoutThreshold.IsNull() {
		m.LockoutThreshold = types.Int64Unknown()
		mutated = true
	}
	if m.MaxAutoDownloadSize.IsNull() {
		m.MaxAutoDownloadSize = types.Int64Unknown()
		mutated = true
	}
	if m.MaxBor.IsNull() {
		m.MaxBor = types.Int64Unknown()
		mutated = true
	}
	if m.MaxTtl.IsNull() {
		m.MaxTtl = types.Int64Unknown()
		mutated = true
	}
	if m.MessageForwardingEnabled.IsNull() {
		m.MessageForwardingEnabled = types.BoolUnknown()
		mutated = true
	}
	if m.PresenceEnabled.IsNull() {
		m.PresenceEnabled = types.BoolUnknown()
		mutated = true
	}
	if m.ShowMasterRecoveryKey.IsNull() {
		m.ShowMasterRecoveryKey = types.BoolUnknown()
		mutated = true
	}
	if m.SsoMaxIdleMinutes.IsNull() {
		m.SsoMaxIdleMinutes = types.Int64Unknown()
		mutated = true
	}
	if m.AtakPackageValues.IsNull() {
		m.AtakPackageValues = fwtypes.NewListValueOfUnknown[types.String](ctx)
		mutated = true
	}
	if m.PermittedNetworks.IsNull() {
		m.PermittedNetworks = fwtypes.NewListValueOfUnknown[types.String](ctx)
		mutated = true
	}
	if m.QuickResponses.IsNull() {
		m.QuickResponses = fwtypes.NewListValueOfUnknown[types.String](ctx)
		mutated = true
	}
	return mutated
}

func (r *securityGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"active_directory_guid": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"active_members": schema.Int64Attribute{
				Computed: true,
			},
			"bot_members": schema.Int64Attribute{
				Computed: true,
			},
			"is_default": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"modified": schema.Int64Attribute{
				Computed: true,
			},
			names.AttrName: schema.StringAttribute{
				Required: true,
			},
			"network_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"security_group_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			// `settings` is a list-of-one nested object declared as a
			// protocol-v5-compatible `schema.ListNestedBlock`. The AWS
			// provider is multiplexed across Plugin SDK v2 (protocol v5)
			// and Plugin Framework (protocol v6); the multiplexer requires
			// every Framework resource's schema to round-trip through v5,
			// and v5 disallows the v6-only `NestedAttributeObject.Attributes`
			// on a nested attribute. Blocks are the v5-native construct
			// for this shape.
			//
			// Drift handling: blocks have no Optional/Computed/
			// PlanModifiers surface, so post-apply consistency is enforced
			// at the flatten layer (`flattenSecurityGroupSettings`) and at
			// the ModifyPlan layer (`hasAnyNullLeaf`) rather than by the
			// schema. See task 6.6 Failure class A.
			"settings": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[securityGroupSettingsModel](ctx),
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: securityGroupSettingsScalarAttributes(),
					Blocks:     securityGroupSettingsNestedBlocks(ctx),
				},
			},
			names.AttrTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *securityGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan securityGroupResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := plan.NetworkID.ValueString()
	name := plan.Name.ValueString()

	// Build CreateSecurityGroup input from the post-ModifyPlan plan.
	// `expandSettingsRequest` emits only fields the user set explicitly
	// in HCL: values rewritten to Unknown by ModifyPlan (see docstring)
	// are treated the same as null values and emit nil on the wire, so
	// AWS's server-side defaults apply.
	sgReq, diags := expandSettingsRequest(ctx, plan.Settings)
	smerr.AddEnrich(ctx, &resp.Diagnostics, diags)
	if resp.Diagnostics.HasError() {
		return
	}

	input := wickr.CreateSecurityGroupInput{
		Name:                  aws.String(name),
		NetworkId:             aws.String(networkID),
		SecurityGroupSettings: sgReq,
	}

	createOut, err := conn.CreateSecurityGroup(ctx, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, enrichPlanTierError(ctx, err, plan.Settings), smerr.ID, compositeID(networkID, name))
		return
	}
	if createOut == nil || createOut.SecurityGroup == nil || createOut.SecurityGroup.Id == nil {
		smerr.AddError(ctx, &resp.Diagnostics, tfresource.NewEmptyResultError(), smerr.ID, compositeID(networkID, name))
		return
	}

	groupID := aws.ToString(createOut.SecurityGroup.Id)

	// If the user configured any Update-only fields, apply them now via
	// UpdateSecurityGroup. Diff the plan against the live state (not
	// an empty base) so only fields the user actually wanted changed
	// go on the wire — AWS rejects full payloads with HTTP 402 and
	// rejects out-of-range scalars with 422.
	if hasUpdateOnlySettings(ctx, plan.Settings) {
		liveSG, err := findSecurityGroupByID(ctx, conn, networkID, groupID)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, groupID))
			return
		}
		liveModel := securityGroupResourceModel{}
		liveModel.Settings = plan.Settings // seed with plan so flatten respects which sub-blocks user set
		if diagsFlatten := flattenSecurityGroup(ctx, liveSG, networkID, &liveModel); diagsFlatten.HasError() {
			smerr.AddEnrich(ctx, &resp.Diagnostics, diagsFlatten)
			return
		}

		sgDiff, expandDiags := expandSettingsDiff(ctx, plan.Settings, liveModel.Settings)
		smerr.AddEnrich(ctx, &resp.Diagnostics, expandDiags)
		if resp.Diagnostics.HasError() {
			return
		}
		if sgDiff != nil {
			input := wickr.UpdateSecurityGroupInput{
				GroupId:               aws.String(groupID),
				NetworkId:             aws.String(networkID),
				Name:                  aws.String(name),
				SecurityGroupSettings: sgDiff,
			}
			if _, err := conn.UpdateSecurityGroup(ctx, &input); err != nil {
				smerr.AddError(ctx, &resp.Diagnostics, enrichPlanTierError(ctx, err, plan.Settings), smerr.ID, compositeID(networkID, groupID))
				return
			}
		}
	}

	// Re-read to pick up the authoritative state (ActiveMembers, BotMembers,
	// Modified, ActiveDirectoryGuid, IsDefault, and every Computed settings
	// leaf not echoed by Create).
	sg, err := findSecurityGroupByID(ctx, conn, networkID, groupID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, groupID))
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, flattenSecurityGroup(ctx, sg, networkID, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *securityGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state securityGroupResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := state.NetworkID.ValueString()
	groupID := state.SecurityGroupID.ValueString()

	sg, err := findSecurityGroupByID(ctx, conn, networkID, groupID)
	if retry.NotFound(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	// Mirror the Network resource: the Wickr service has been observed to
	// return *awstypes.ForbiddenError during delete-in-progress windows.
	// Additionally, when the parent network has been deleted out-of-band
	// (the common case when the `disappears` test hits `DeleteNetwork`
	// which cascades to children), Wickr's `GetSecurityGroup` against the
	// orphaned SG returns an HTTP 401 page whose HTML-shaped body the SDK
	// decoder fails to parse, surfacing as a smithy error with
	// "deserialization failed, failed to decode response body, invalid
	// character 'i' looking for beginning of value". In either case,
	// treat as "gone" so `terraform refresh` cleanly removes the resource.
	if isSecurityGroupOrphanedChildError(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, groupID))
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, flattenSecurityGroup(ctx, sg, networkID, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &state))
}

func (r *securityGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state securityGroupResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := state.NetworkID.ValueString()
	groupID := state.SecurityGroupID.ValueString()
	name := plan.Name.ValueString()

	// Compute a sparse `SecurityGroupSettings` diff (plan − state): only
	// fields whose value actually changed are included. AWS Wickr's
	// `UpdateSecurityGroup` rejects full echoed-back payloads with
	// HTTP 402 and rejects out-of-range scalars with 422. Sending only
	// the changed fields sidesteps both.
	sgDiff, expandDiags := expandSettingsDiff(ctx, plan.Settings, state.Settings)
	smerr.AddEnrich(ctx, &resp.Diagnostics, expandDiags)
	if resp.Diagnostics.HasError() {
		return
	}

	nameChanged := !plan.Name.Equal(state.Name)
	if sgDiff != nil || nameChanged {
		input := wickr.UpdateSecurityGroupInput{
			GroupId:   aws.String(groupID),
			NetworkId: aws.String(networkID),
		}
		if nameChanged {
			input.Name = aws.String(name)
		}
		if sgDiff != nil {
			input.SecurityGroupSettings = sgDiff
		}
		_, err := conn.UpdateSecurityGroup(ctx, &input)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, enrichPlanTierError(ctx, err, plan.Settings), smerr.ID, compositeID(networkID, groupID))
			return
		}
	}

	sg, err := findSecurityGroupByID(ctx, conn, networkID, groupID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, groupID))
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, flattenSecurityGroup(ctx, sg, networkID, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *securityGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state securityGroupResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := state.NetworkID.ValueString()
	groupID := state.SecurityGroupID.ValueString()

	// Default security groups are created automatically by
	// CreateNetwork and `DeleteSecurityGroup` rejects them server-side
	// ("This operation cannot be performed on the default security
	// group", per SDK doc). Users should manage the default SG via a
	// future `aws_wickr_default_security_group` resource that adopts
	// the existing object and treats destroy as a state-only no-op
	// (mirroring `aws_default_security_group` in the VPC service). A
	// user who explicitly imports a default SG into this resource and
	// then destroys it will see the raw AWS error, which is the
	// correct signal that they're using the wrong resource type.

	input := wickr.DeleteSecurityGroupInput{
		GroupId:   aws.String(groupID),
		NetworkId: aws.String(networkID),
	}
	_, err := conn.DeleteSecurityGroup(ctx, &input)
	// Treat already-gone responses as success. Three shapes are expected here:
	//   1. *awstypes.ResourceNotFoundError — the SG has been deleted.
	//   2. *awstypes.ForbiddenError — the parent network is in
	//      delete-in-progress and will cascade-delete this SG.
	//   3. Deserialization failure on an HTTP 401 HTML page — the parent
	//      network has fully vanished and the orphaned-child path returns
	//      an HTML error page that the SDK decoder cannot parse.
	if errs.IsA[*awstypes.ResourceNotFoundError](err) || isSecurityGroupOrphanedChildError(err) {
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, groupID))
		return
	}
}

// isSecurityGroupOrphanedChildError returns true when `err` indicates that
// the security group is gone (or its parent network is) for any of the
// three reasons the Wickr service surfaces:
//   - *awstypes.ForbiddenError: parent network is in delete-in-progress.
//   - Smithy HTTP 401 deserialization failure: parent network has fully
//     vanished and Wickr returns an HTML error page that the SDK decoder
//     cannot parse ("deserialization failed, failed to decode response
//     body"). Observed in acceptance tests when `disappears` deletes the
//     parent network mid-test.
func isSecurityGroupOrphanedChildError(err error) bool {
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

// compositeID returns the `(network_id, security_group_id)` identifier in
// the `,`-separated form used throughout the Wickr provider for
// parameterized identity (per docs/id-attributes.md and design.md).
func compositeID(networkID, groupID string) string {
	return fmt.Sprintf("%s,%s", networkID, groupID)
}

// securityGroupResourceModel mirrors the Framework-resource schema. Fields
// match the schema attribute keys exactly via `tfsdk` tags. The `Settings`
// nested object is materialized from `types.SecurityGroup.SecurityGroupSettings`
// on Read, and split into Create-settable vs Update-settable halves on
// Create/Update (see design.md).
type securityGroupResourceModel struct {
	framework.WithRegionModel
	ActiveDirectoryGUID types.String                                                `tfsdk:"active_directory_guid"`
	ActiveMembers       types.Int64                                                 `tfsdk:"active_members"`
	BotMembers          types.Int64                                                 `tfsdk:"bot_members"`
	IsDefault           types.Bool                                                  `tfsdk:"is_default"`
	Modified            types.Int64                                                 `tfsdk:"modified"`
	Name                types.String                                                `tfsdk:"name"`
	NetworkID           types.String                                                `tfsdk:"network_id"`
	SecurityGroupID     types.String                                                `tfsdk:"security_group_id"`
	Settings            fwtypes.ListNestedObjectValueOf[securityGroupSettingsModel] `tfsdk:"settings"`
	Timeouts            timeouts.Value                                              `tfsdk:"timeouts"`
}

// securityGroupSettingsModel is the full-fat settings object: every field
// that appears on `types.SecurityGroupSettings`. The Create handler only
// sends the Create-settable subset; every other knob is applied via a
// follow-up UpdateSecurityGroup call (see `hasUpdateOnlySettings` and
// `expandSettingsRequest`).
type securityGroupSettingsModel struct {
	AlwaysReauthenticate             types.Bool                                                   `tfsdk:"always_reauthenticate"`
	AtakPackageValues                fwtypes.ListValueOf[types.String]                            `tfsdk:"atak_package_values"`
	Calling                          fwtypes.ListNestedObjectValueOf[callingSettingsModel]        `tfsdk:"calling"`
	CheckForUpdates                  types.Bool                                                   `tfsdk:"check_for_updates"`
	EnableAtak                       types.Bool                                                   `tfsdk:"enable_atak"`
	EnableCrashReports               types.Bool                                                   `tfsdk:"enable_crash_reports"`
	EnableFileDownload               types.Bool                                                   `tfsdk:"enable_file_download"`
	EnableGuestFederation            types.Bool                                                   `tfsdk:"enable_guest_federation"`
	EnableNotificationPreview        types.Bool                                                   `tfsdk:"enable_notification_preview"`
	EnableOpenAccessOption           types.Bool                                                   `tfsdk:"enable_open_access_option"`
	EnableRestrictedGlobalFederation types.Bool                                                   `tfsdk:"enable_restricted_global_federation"`
	FederationMode                   types.Int64                                                  `tfsdk:"federation_mode"`
	FilesEnabled                     types.Bool                                                   `tfsdk:"files_enabled"`
	ForceDeviceLockout               types.Int64                                                  `tfsdk:"force_device_lockout"`
	ForceOpenAccess                  types.Bool                                                   `tfsdk:"force_open_access"`
	ForceReadReceipts                types.Bool                                                   `tfsdk:"force_read_receipts"`
	GlobalFederation                 types.Bool                                                   `tfsdk:"global_federation"`
	IsAtoEnabled                     types.Bool                                                   `tfsdk:"is_ato_enabled"`
	IsLinkPreviewEnabled             types.Bool                                                   `tfsdk:"is_link_preview_enabled"`
	LocationAllowMaps                types.Bool                                                   `tfsdk:"location_allow_maps"`
	LocationEnabled                  types.Bool                                                   `tfsdk:"location_enabled"`
	LockoutThreshold                 types.Int64                                                  `tfsdk:"lockout_threshold"`
	MaxAutoDownloadSize              types.Int64                                                  `tfsdk:"max_auto_download_size"`
	MaxBor                           types.Int64                                                  `tfsdk:"max_bor"`
	MaxTtl                           types.Int64                                                  `tfsdk:"max_ttl"`
	MessageForwardingEnabled         types.Bool                                                   `tfsdk:"message_forwarding_enabled"`
	PasswordRequirements             fwtypes.ListNestedObjectValueOf[passwordRequirementsModel]   `tfsdk:"password_requirements"`
	PermittedNetworks                fwtypes.ListValueOf[types.String]                            `tfsdk:"permitted_networks"`
	PermittedWickrAwsNetworks        fwtypes.ListNestedObjectValueOf[wickrAwsNetworkModel]        `tfsdk:"permitted_wickr_aws_networks"`
	PermittedWickrEnterpriseNetworks fwtypes.ListNestedObjectValueOf[wickrEnterpriseNetworkModel] `tfsdk:"permitted_wickr_enterprise_networks"`
	PresenceEnabled                  types.Bool                                                   `tfsdk:"presence_enabled"`
	QuickResponses                   fwtypes.ListValueOf[types.String]                            `tfsdk:"quick_responses"`
	ShowMasterRecoveryKey            types.Bool                                                   `tfsdk:"show_master_recovery_key"`
	Shredder                         fwtypes.ListNestedObjectValueOf[shredderSettingsModel]       `tfsdk:"shredder"`
	SsoMaxIdleMinutes                types.Int64                                                  `tfsdk:"sso_max_idle_minutes"`
}

type callingSettingsModel struct {
	CanStart11Call types.Bool `tfsdk:"can_start_11_call"`
	CanVideoCall   types.Bool `tfsdk:"can_video_call"`
	ForceTcpCall   types.Bool `tfsdk:"force_tcp_call"`
}

type passwordRequirementsModel struct {
	Lowercase types.Int64 `tfsdk:"lowercase"`
	MinLength types.Int64 `tfsdk:"min_length"`
	Numbers   types.Int64 `tfsdk:"numbers"`
	Symbols   types.Int64 `tfsdk:"symbols"`
	Uppercase types.Int64 `tfsdk:"uppercase"`
}

type shredderSettingsModel struct {
	CanProcessManually types.Bool  `tfsdk:"can_process_manually"`
	Intensity          types.Int64 `tfsdk:"intensity"`
}

type wickrAwsNetworkModel struct {
	NetworkID types.String `tfsdk:"network_id"`
	Region    types.String `tfsdk:"region"`
}

type wickrEnterpriseNetworkModel struct {
	Domain    types.String `tfsdk:"domain"`
	NetworkID types.String `tfsdk:"network_id"`
}

// hasUpdateOnlySettings reports whether the plan's `settings` block
// contains any knob that is NOT writable via CreateSecurityGroup (i.e., one
// of the Update-only fields from the SDK's `SecurityGroupSettings` but
// absent from `SecurityGroupSettingsRequest`). Returning `true` causes the
// Create handler to issue a follow-up UpdateSecurityGroup call so the
// complete intended state reaches AWS after a single apply.
func hasUpdateOnlySettings(ctx context.Context, v fwtypes.ListNestedObjectValueOf[securityGroupSettingsModel]) bool {
	m, diags := v.ToPtr(ctx)
	if diags.HasError() || m == nil {
		return false
	}
	// The Create-settable fields (per design.md → Create-settable column):
	//   EnableGuestFederation, EnableRestrictedGlobalFederation,
	//   FederationMode, GlobalFederation, LockoutThreshold, PermittedNetworks,
	//   PermittedWickrAwsNetworks, PermittedWickrEnterpriseNetworks.
	// Every other field is Update-only. If the user configured any of the
	// Update-only fields (i.e., the value is non-null and non-unknown), we
	// need a follow-up Update.
	if !m.AlwaysReauthenticate.IsNull() && !m.AlwaysReauthenticate.IsUnknown() {
		return true
	}
	if !m.AtakPackageValues.IsNull() && !m.AtakPackageValues.IsUnknown() {
		return true
	}
	if !m.Calling.IsNull() && !m.Calling.IsUnknown() {
		return true
	}
	if !m.CheckForUpdates.IsNull() && !m.CheckForUpdates.IsUnknown() {
		return true
	}
	if !m.EnableAtak.IsNull() && !m.EnableAtak.IsUnknown() {
		return true
	}
	if !m.EnableCrashReports.IsNull() && !m.EnableCrashReports.IsUnknown() {
		return true
	}
	if !m.EnableFileDownload.IsNull() && !m.EnableFileDownload.IsUnknown() {
		return true
	}
	if !m.EnableNotificationPreview.IsNull() && !m.EnableNotificationPreview.IsUnknown() {
		return true
	}
	if !m.EnableOpenAccessOption.IsNull() && !m.EnableOpenAccessOption.IsUnknown() {
		return true
	}
	if !m.FilesEnabled.IsNull() && !m.FilesEnabled.IsUnknown() {
		return true
	}
	if !m.ForceDeviceLockout.IsNull() && !m.ForceDeviceLockout.IsUnknown() {
		return true
	}
	if !m.ForceOpenAccess.IsNull() && !m.ForceOpenAccess.IsUnknown() {
		return true
	}
	if !m.ForceReadReceipts.IsNull() && !m.ForceReadReceipts.IsUnknown() {
		return true
	}
	if !m.IsAtoEnabled.IsNull() && !m.IsAtoEnabled.IsUnknown() {
		return true
	}
	if !m.IsLinkPreviewEnabled.IsNull() && !m.IsLinkPreviewEnabled.IsUnknown() {
		return true
	}
	if !m.LocationAllowMaps.IsNull() && !m.LocationAllowMaps.IsUnknown() {
		return true
	}
	if !m.LocationEnabled.IsNull() && !m.LocationEnabled.IsUnknown() {
		return true
	}
	if !m.MaxAutoDownloadSize.IsNull() && !m.MaxAutoDownloadSize.IsUnknown() {
		return true
	}
	if !m.MaxBor.IsNull() && !m.MaxBor.IsUnknown() {
		return true
	}
	if !m.MaxTtl.IsNull() && !m.MaxTtl.IsUnknown() {
		return true
	}
	if !m.MessageForwardingEnabled.IsNull() && !m.MessageForwardingEnabled.IsUnknown() {
		return true
	}
	if !m.PasswordRequirements.IsNull() && !m.PasswordRequirements.IsUnknown() {
		return true
	}
	if !m.PresenceEnabled.IsNull() && !m.PresenceEnabled.IsUnknown() {
		return true
	}
	if !m.QuickResponses.IsNull() && !m.QuickResponses.IsUnknown() {
		return true
	}
	if !m.ShowMasterRecoveryKey.IsNull() && !m.ShowMasterRecoveryKey.IsUnknown() {
		return true
	}
	if !m.Shredder.IsNull() && !m.Shredder.IsUnknown() {
		return true
	}
	if !m.SsoMaxIdleMinutes.IsNull() && !m.SsoMaxIdleMinutes.IsUnknown() {
		return true
	}
	return false
}

// nullSafeInt32 returns nil when the framework value is null or unknown;
// otherwise returns `Int32FromFrameworkInt64`. This is required because AWS
// Wickr's `UpdateSecurityGroup` rejects every `*int32(0)` field with an
// opaque `BadRequestError:”` — it treats zero as "no value", but the SDK
// field is a non-pointer struct field set by
// `Int32FromFrameworkInt64` on a null Int64 (which is 0). See task 6.6
// Failure class D for full context.
func nullSafeInt32(ctx context.Context, v types.Int64) *int32 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return fwflex.Int32FromFrameworkInt64(ctx, v)
}

// nullSafeInt64 is the `*int64` counterpart to `nullSafeInt32`; a handful of
// settings leaves (MaxAutoDownloadSize, MaxTtl) are typed `*int64` on the
// SDK shape instead of `*int32`.
func nullSafeInt64(ctx context.Context, v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return fwflex.Int64FromFramework(ctx, v)
}

// nullSafeBool mirrors `nullSafeInt32` for boolean settings leaves.
func nullSafeBool(ctx context.Context, v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return fwflex.BoolFromFramework(ctx, v)
}

// expandSettingsRequest builds a `*types.SecurityGroupSettingsRequest` from
// the plan's `settings` block. Only the Create-settable fields are read
// (EnableGuestFederation, EnableRestrictedGlobalFederation, FederationMode,
// GlobalFederation, LockoutThreshold, PermittedNetworks,
// PermittedWickrAwsNetworks, PermittedWickrEnterpriseNetworks).
//
// AutoFlex is not usable here because `SecurityGroupSettingsRequest` is a
// strict Create-time subset of `SecurityGroupSettings` (see design.md →
// "`aws_wickr_security_group` resource → CRUD pseudocode"): the Terraform
// model is the union, and AutoFlex cannot split one model into two SDK
// shapes. The hand-written split is deliberate.
//
// Every scalar leaf is built via `nullSafeInt32` / `nullSafeBool` so we
// never emit `*int32(0)` / `*bool(false)` for fields the user left
// unconfigured — AWS Wickr rejects explicit zero/false on some settings
// fields (task 6.6 Failure class D).
//
// nosemgrep:ci.semgrep.framework.manual-expander-functions
func expandSettingsRequest(ctx context.Context, v fwtypes.ListNestedObjectValueOf[securityGroupSettingsModel]) (*awstypes.SecurityGroupSettingsRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	m, d := v.ToPtr(ctx)
	diags.Append(d...)
	if diags.HasError() || m == nil {
		// `settings` is Optional+Computed in the schema, so ToPtr returning
		// nil indicates an omitted plan. `CreateSecurityGroupInput.SecurityGroupSettings`
		// is marked required by the SDK (verified 2026-04-19 against live
		// us-east-1: omitting it entirely returns "1 validation error(s)
		// found. - missing required field,
		// CreateSecurityGroupInput.SecurityGroupSettings."). Send an empty
		// `SecurityGroupSettingsRequest{}` so every field defaults
		// server-side.
		return &awstypes.SecurityGroupSettingsRequest{}, diags
	}

	out := awstypes.SecurityGroupSettingsRequest{
		EnableGuestFederation:            nullSafeBool(ctx, m.EnableGuestFederation),
		EnableRestrictedGlobalFederation: nullSafeBool(ctx, m.EnableRestrictedGlobalFederation),
		FederationMode:                   nullSafeInt32(ctx, m.FederationMode),
		GlobalFederation:                 nullSafeBool(ctx, m.GlobalFederation),
		LockoutThreshold:                 nullSafeInt32(ctx, m.LockoutThreshold),
	}

	if !m.PermittedNetworks.IsNull() && !m.PermittedNetworks.IsUnknown() {
		var s []string
		d := m.PermittedNetworks.ElementsAs(ctx, &s, false)
		diags.Append(d...)
		out.PermittedNetworks = s
	}

	if !m.PermittedWickrAwsNetworks.IsNull() && !m.PermittedWickrAwsNetworks.IsUnknown() {
		elems, d := m.PermittedWickrAwsNetworks.ToSlice(ctx)
		diags.Append(d...)
		for _, e := range elems {
			if e == nil {
				continue
			}
			out.PermittedWickrAwsNetworks = append(out.PermittedWickrAwsNetworks, awstypes.WickrAwsNetworks{
				NetworkId: e.NetworkID.ValueStringPointer(),
				Region:    e.Region.ValueStringPointer(),
			})
		}
	}

	if !m.PermittedWickrEnterpriseNetworks.IsNull() && !m.PermittedWickrEnterpriseNetworks.IsUnknown() {
		elems, d := m.PermittedWickrEnterpriseNetworks.ToSlice(ctx)
		diags.Append(d...)
		for _, e := range elems {
			if e == nil {
				continue
			}
			out.PermittedWickrEnterpriseNetworks = append(out.PermittedWickrEnterpriseNetworks, awstypes.PermittedWickrEnterpriseNetwork{
				Domain:    e.Domain.ValueStringPointer(),
				NetworkId: e.NetworkID.ValueStringPointer(),
			})
		}
	}

	return &out, diags
}

// expandSettingsDiff builds a sparse `*types.SecurityGroupSettings`
// containing only fields whose value differs between plan and state.
// AWS Wickr's `UpdateSecurityGroup` rejects echoed-back full settings
// payloads with HTTP 402 ("ParameterUnchanged") — sparse is required.
//
// Returns (nil, diags) when no scalar/list leaf differs; callers should
// skip the API call entirely in that case.
//
// nosemgrep:ci.semgrep.framework.manual-expander-functions
func expandSettingsDiff(ctx context.Context, plan, state fwtypes.ListNestedObjectValueOf[securityGroupSettingsModel]) (*awstypes.SecurityGroupSettings, diag.Diagnostics) {
	var diags diag.Diagnostics

	pm, d := plan.ToPtr(ctx)
	diags.Append(d...)
	if diags.HasError() || pm == nil {
		return nil, diags
	}

	var sm *securityGroupSettingsModel
	if !state.IsNull() && !state.IsUnknown() {
		sm, d = state.ToPtr(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
	}

	out := awstypes.SecurityGroupSettings{}
	changed := false

	// Scalars: for each leaf, emit only if plan differs from state.
	// A changed-but-null-in-plan leaf skips the emit (AWS rejects null
	// unsets via the "invalid options or data" 402; we keep prior state
	// in AWS unchanged by not mentioning the field).
	setBool := func(field **bool, planVal types.Bool, stateVal types.Bool) {
		if planVal.Equal(stateVal) || planVal.IsNull() || planVal.IsUnknown() {
			return
		}
		*field = nullSafeBool(ctx, planVal)
		changed = true
	}
	setInt32 := func(field **int32, planVal types.Int64, stateVal types.Int64) {
		if planVal.Equal(stateVal) || planVal.IsNull() || planVal.IsUnknown() {
			return
		}
		*field = nullSafeInt32(ctx, planVal)
		changed = true
	}
	setInt64 := func(field **int64, planVal types.Int64, stateVal types.Int64) {
		if planVal.Equal(stateVal) || planVal.IsNull() || planVal.IsUnknown() {
			return
		}
		*field = nullSafeInt64(ctx, planVal)
		changed = true
	}

	var empty securityGroupSettingsModel
	if sm == nil {
		sm = &empty
	}

	setBool(&out.AlwaysReauthenticate, pm.AlwaysReauthenticate, sm.AlwaysReauthenticate)
	setBool(&out.CheckForUpdates, pm.CheckForUpdates, sm.CheckForUpdates)
	setBool(&out.EnableAtak, pm.EnableAtak, sm.EnableAtak)
	setBool(&out.EnableCrashReports, pm.EnableCrashReports, sm.EnableCrashReports)
	setBool(&out.EnableFileDownload, pm.EnableFileDownload, sm.EnableFileDownload)
	setBool(&out.EnableGuestFederation, pm.EnableGuestFederation, sm.EnableGuestFederation)
	setBool(&out.EnableNotificationPreview, pm.EnableNotificationPreview, sm.EnableNotificationPreview)
	setBool(&out.EnableOpenAccessOption, pm.EnableOpenAccessOption, sm.EnableOpenAccessOption)
	setBool(&out.EnableRestrictedGlobalFederation, pm.EnableRestrictedGlobalFederation, sm.EnableRestrictedGlobalFederation)
	setInt32(&out.FederationMode, pm.FederationMode, sm.FederationMode)
	setBool(&out.FilesEnabled, pm.FilesEnabled, sm.FilesEnabled)
	setInt32(&out.ForceDeviceLockout, pm.ForceDeviceLockout, sm.ForceDeviceLockout)
	setBool(&out.ForceOpenAccess, pm.ForceOpenAccess, sm.ForceOpenAccess)
	setBool(&out.ForceReadReceipts, pm.ForceReadReceipts, sm.ForceReadReceipts)
	setBool(&out.GlobalFederation, pm.GlobalFederation, sm.GlobalFederation)
	setBool(&out.IsAtoEnabled, pm.IsAtoEnabled, sm.IsAtoEnabled)
	setBool(&out.IsLinkPreviewEnabled, pm.IsLinkPreviewEnabled, sm.IsLinkPreviewEnabled)
	setBool(&out.LocationAllowMaps, pm.LocationAllowMaps, sm.LocationAllowMaps)
	setBool(&out.LocationEnabled, pm.LocationEnabled, sm.LocationEnabled)
	setInt32(&out.LockoutThreshold, pm.LockoutThreshold, sm.LockoutThreshold)
	setInt64(&out.MaxAutoDownloadSize, pm.MaxAutoDownloadSize, sm.MaxAutoDownloadSize)
	setInt32(&out.MaxBor, pm.MaxBor, sm.MaxBor)
	setInt64(&out.MaxTtl, pm.MaxTtl, sm.MaxTtl)
	setBool(&out.MessageForwardingEnabled, pm.MessageForwardingEnabled, sm.MessageForwardingEnabled)
	setBool(&out.PresenceEnabled, pm.PresenceEnabled, sm.PresenceEnabled)
	setBool(&out.ShowMasterRecoveryKey, pm.ShowMasterRecoveryKey, sm.ShowMasterRecoveryKey)
	setInt32(&out.SsoMaxIdleMinutes, pm.SsoMaxIdleMinutes, sm.SsoMaxIdleMinutes)

	// List-of-strings: emit only if plan value differs from state.
	if !pm.AtakPackageValues.Equal(sm.AtakPackageValues) && !pm.AtakPackageValues.IsNull() && !pm.AtakPackageValues.IsUnknown() {
		var s []string
		dd := pm.AtakPackageValues.ElementsAs(ctx, &s, false)
		diags.Append(dd...)
		out.AtakPackageValues = s
		changed = true
	}
	if !pm.PermittedNetworks.Equal(sm.PermittedNetworks) && !pm.PermittedNetworks.IsNull() && !pm.PermittedNetworks.IsUnknown() {
		var s []string
		dd := pm.PermittedNetworks.ElementsAs(ctx, &s, false)
		diags.Append(dd...)
		out.PermittedNetworks = s
		changed = true
	}
	if !pm.QuickResponses.Equal(sm.QuickResponses) && !pm.QuickResponses.IsNull() && !pm.QuickResponses.IsUnknown() {
		var s []string
		dd := pm.QuickResponses.ElementsAs(ctx, &s, false)
		diags.Append(dd...)
		out.QuickResponses = s
		changed = true
	}

	// Nested sub-blocks (permitted_wickr_*_networks) — calling/
	// password_requirements/shredder are schema-blocked today (see
	// `listvalidator.SizeAtMost(0)`).
	if !pm.PermittedWickrAwsNetworks.Equal(sm.PermittedWickrAwsNetworks) &&
		!pm.PermittedWickrAwsNetworks.IsNull() && !pm.PermittedWickrAwsNetworks.IsUnknown() {
		elems, dd := pm.PermittedWickrAwsNetworks.ToSlice(ctx)
		diags.Append(dd...)
		for _, e := range elems {
			if e == nil {
				continue
			}
			out.PermittedWickrAwsNetworks = append(out.PermittedWickrAwsNetworks, awstypes.WickrAwsNetworks{
				NetworkId: e.NetworkID.ValueStringPointer(),
				Region:    e.Region.ValueStringPointer(),
			})
		}
		changed = true
	}
	if !pm.PermittedWickrEnterpriseNetworks.Equal(sm.PermittedWickrEnterpriseNetworks) &&
		!pm.PermittedWickrEnterpriseNetworks.IsNull() && !pm.PermittedWickrEnterpriseNetworks.IsUnknown() {
		elems, dd := pm.PermittedWickrEnterpriseNetworks.ToSlice(ctx)
		diags.Append(dd...)
		for _, e := range elems {
			if e == nil {
				continue
			}
			out.PermittedWickrEnterpriseNetworks = append(out.PermittedWickrEnterpriseNetworks, awstypes.PermittedWickrEnterpriseNetwork{
				Domain:    e.Domain.ValueStringPointer(),
				NetworkId: e.NetworkID.ValueStringPointer(),
			})
		}
		changed = true
	}

	if !changed {
		return nil, diags
	}
	return &out, diags
}

// flattenSecurityGroup materializes a `*types.SecurityGroup` into the
// Framework model. The `network_id` is passed in separately because
// `types.SecurityGroup` has no back-reference to its parent network.
//
// The caller's existing `m` is inspected to preserve null settings (and
// null nested sub-blocks) that the user didn't write in HCL. AWS always
// returns a fully populated `SecurityGroupSettings` object, but users
// who omitted `settings {}` or a sub-block expect their plan's null
// blocks to survive into state. This function respects that intent.
//
// AutoFlex is not usable here: `types.SecurityGroup` has no NetworkId field
// to flatten (the parent network is inferred by URL, not echoed), and the
// nested `SecurityGroupSettings` contains a large union of Create-vs-Update
// shapes that the Terraform model flattens as a single structured block.
// See design.md → "`aws_wickr_security_group` resource".
//
// nosemgrep:ci.semgrep.framework.manual-flattener-functions
func flattenSecurityGroup(ctx context.Context, sg *awstypes.SecurityGroup, networkID string, m *securityGroupResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.NetworkID = types.StringValue(networkID)
	m.ActiveDirectoryGUID = fwflex.StringToFramework(ctx, sg.ActiveDirectoryGuid)
	m.ActiveMembers = fwflex.Int32ToFrameworkInt64(ctx, sg.ActiveMembers)
	m.BotMembers = fwflex.Int32ToFrameworkInt64(ctx, sg.BotMembers)
	m.IsDefault = fwflex.BoolToFramework(ctx, sg.IsDefault)
	m.Modified = fwflex.Int32ToFrameworkInt64(ctx, sg.Modified)
	m.Name = fwflex.StringToFramework(ctx, sg.Name)
	m.SecurityGroupID = fwflex.StringToFramework(ctx, sg.Id)

	// Preserve the user's omit-or-include choice for the outer
	// `settings` block. If the caller's prior model had null settings
	// (the user wrote no `settings {}` in HCL), AWS's response is
	// dropped to keep the block count at 0.
	if m.Settings.IsNull() {
		return diags
	}

	// Pull prior nested sub-block null-ness BEFORE overwriting m.Settings,
	// so the inner flatten can match AWS's response against the user's
	// HCL intent.
	prior := priorSubBlocks{
		calling:                          true,
		passwordRequirements:             true,
		shredder:                         true,
		permittedWickrAwsNetworks:        true,
		permittedWickrEnterpriseNetworks: true,
	}
	if !m.Settings.IsUnknown() {
		if priorModel, d := m.Settings.ToPtr(ctx); !d.HasError() && priorModel != nil {
			prior.calling = priorModel.Calling.IsNull()
			prior.passwordRequirements = priorModel.PasswordRequirements.IsNull()
			prior.shredder = priorModel.Shredder.IsNull()
			prior.permittedWickrAwsNetworks = priorModel.PermittedWickrAwsNetworks.IsNull() ||
				(!priorModel.PermittedWickrAwsNetworks.IsUnknown() && len(mustSlice(ctx, priorModel.PermittedWickrAwsNetworks)) == 0)
			prior.permittedWickrEnterpriseNetworks = priorModel.PermittedWickrEnterpriseNetworks.IsNull() ||
				(!priorModel.PermittedWickrEnterpriseNetworks.IsUnknown() && len(mustSlice(ctx, priorModel.PermittedWickrEnterpriseNetworks)) == 0)
		}
	}

	settings, d := flattenSecurityGroupSettings(ctx, sg.SecurityGroupSettings, prior)
	diags.Append(d...)
	m.Settings = settings

	return diags
}

// priorSubBlocks captures the null-ness of the five nested sub-blocks
// from the caller's prior model, so the flatten layer can preserve the
// user's omit-or-include choice.
type priorSubBlocks struct {
	calling                          bool // true if prior block was null or absent
	passwordRequirements             bool
	shredder                         bool
	permittedWickrAwsNetworks        bool
	permittedWickrEnterpriseNetworks bool
}

// mustSlice is a small helper that swallows diagnostics from
// ToSlice — used by the prior-sub-block inspection where we only care
// about the element count.
func mustSlice[T any](ctx context.Context, v fwtypes.ListNestedObjectValueOf[T]) []*T {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	slice, _ := v.ToSlice(ctx)
	return slice
}

// nosemgrep:ci.semgrep.framework.manual-flattener-functions
func flattenSecurityGroupSettings(ctx context.Context, s *awstypes.SecurityGroupSettings, prior priorSubBlocks) (fwtypes.ListNestedObjectValueOf[securityGroupSettingsModel], diag.Diagnostics) {
	var diags diag.Diagnostics

	if s == nil {
		return fwtypes.NewListNestedObjectValueOfNull[securityGroupSettingsModel](ctx), diags
	}

	m := securityGroupSettingsModel{
		AlwaysReauthenticate:             fwflex.BoolToFramework(ctx, s.AlwaysReauthenticate),
		CheckForUpdates:                  fwflex.BoolToFramework(ctx, s.CheckForUpdates),
		EnableAtak:                       fwflex.BoolToFramework(ctx, s.EnableAtak),
		EnableCrashReports:               fwflex.BoolToFramework(ctx, s.EnableCrashReports),
		EnableFileDownload:               fwflex.BoolToFramework(ctx, s.EnableFileDownload),
		EnableGuestFederation:            fwflex.BoolToFramework(ctx, s.EnableGuestFederation),
		EnableNotificationPreview:        fwflex.BoolToFramework(ctx, s.EnableNotificationPreview),
		EnableOpenAccessOption:           fwflex.BoolToFramework(ctx, s.EnableOpenAccessOption),
		EnableRestrictedGlobalFederation: fwflex.BoolToFramework(ctx, s.EnableRestrictedGlobalFederation),
		FederationMode:                   fwflex.Int32ToFrameworkInt64(ctx, s.FederationMode),
		FilesEnabled:                     fwflex.BoolToFramework(ctx, s.FilesEnabled),
		ForceDeviceLockout:               fwflex.Int32ToFrameworkInt64(ctx, s.ForceDeviceLockout),
		ForceOpenAccess:                  fwflex.BoolToFramework(ctx, s.ForceOpenAccess),
		ForceReadReceipts:                fwflex.BoolToFramework(ctx, s.ForceReadReceipts),
		GlobalFederation:                 fwflex.BoolToFramework(ctx, s.GlobalFederation),
		IsAtoEnabled:                     fwflex.BoolToFramework(ctx, s.IsAtoEnabled),
		IsLinkPreviewEnabled:             fwflex.BoolToFramework(ctx, s.IsLinkPreviewEnabled),
		LocationAllowMaps:                fwflex.BoolToFramework(ctx, s.LocationAllowMaps),
		LocationEnabled:                  fwflex.BoolToFramework(ctx, s.LocationEnabled),
		LockoutThreshold:                 fwflex.Int32ToFrameworkInt64(ctx, s.LockoutThreshold),
		MaxAutoDownloadSize:              fwflex.Int64ToFramework(ctx, s.MaxAutoDownloadSize),
		MaxBor:                           fwflex.Int32ToFrameworkInt64(ctx, s.MaxBor),
		MaxTtl:                           fwflex.Int64ToFramework(ctx, s.MaxTtl),
		MessageForwardingEnabled:         fwflex.BoolToFramework(ctx, s.MessageForwardingEnabled),
		PresenceEnabled:                  fwflex.BoolToFramework(ctx, s.PresenceEnabled),
		ShowMasterRecoveryKey:            fwflex.BoolToFramework(ctx, s.ShowMasterRecoveryKey),
		SsoMaxIdleMinutes:                fwflex.Int32ToFrameworkInt64(ctx, s.SsoMaxIdleMinutes),
	}

	m.AtakPackageValues = flattenStringSliceToListOfString(ctx, s.AtakPackageValues)
	m.PermittedNetworks = flattenStringSliceToListOfString(ctx, s.PermittedNetworks)
	m.QuickResponses = flattenStringSliceToListOfString(ctx, s.QuickResponses)

	// Nested sub-blocks: respect the user's omit-or-include choice
	// recorded in `prior`. If the prior model had the sub-block null
	// (user wrote no `calling {}`, etc.), keep it null in the output
	// even though AWS's response carries values for it. This is what
	// makes `settings { federation_mode = 1 }` survive Create without
	// tripping a per-sub-block consistency check.
	if s.Calling != nil && !prior.calling {
		c := callingSettingsModel{
			CanStart11Call: fwflex.BoolToFramework(ctx, s.Calling.CanStart11Call),
			CanVideoCall:   fwflex.BoolToFramework(ctx, s.Calling.CanVideoCall),
			ForceTcpCall:   fwflex.BoolToFramework(ctx, s.Calling.ForceTcpCall),
		}
		v, d := fwtypes.NewListNestedObjectValueOfPtr(ctx, &c)
		diags.Append(d...)
		m.Calling = v
	} else {
		m.Calling = fwtypes.NewListNestedObjectValueOfNull[callingSettingsModel](ctx)
	}

	if s.PasswordRequirements != nil && !prior.passwordRequirements {
		p := passwordRequirementsModel{
			Lowercase: fwflex.Int32ToFrameworkInt64(ctx, s.PasswordRequirements.Lowercase),
			MinLength: fwflex.Int32ToFrameworkInt64(ctx, s.PasswordRequirements.MinLength),
			Numbers:   fwflex.Int32ToFrameworkInt64(ctx, s.PasswordRequirements.Numbers),
			Symbols:   fwflex.Int32ToFrameworkInt64(ctx, s.PasswordRequirements.Symbols),
			Uppercase: fwflex.Int32ToFrameworkInt64(ctx, s.PasswordRequirements.Uppercase),
		}
		v, d := fwtypes.NewListNestedObjectValueOfPtr(ctx, &p)
		diags.Append(d...)
		m.PasswordRequirements = v
	} else {
		m.PasswordRequirements = fwtypes.NewListNestedObjectValueOfNull[passwordRequirementsModel](ctx)
	}

	if s.Shredder != nil && !prior.shredder {
		sh := shredderSettingsModel{
			CanProcessManually: fwflex.BoolToFramework(ctx, s.Shredder.CanProcessManually),
			Intensity:          fwflex.Int32ToFrameworkInt64(ctx, s.Shredder.Intensity),
		}
		v, d := fwtypes.NewListNestedObjectValueOfPtr(ctx, &sh)
		diags.Append(d...)
		m.Shredder = v
	} else {
		m.Shredder = fwtypes.NewListNestedObjectValueOfNull[shredderSettingsModel](ctx)
	}

	if len(s.PermittedWickrAwsNetworks) > 0 && !prior.permittedWickrAwsNetworks {
		elems := make([]*wickrAwsNetworkModel, 0, len(s.PermittedWickrAwsNetworks))
		for i := range s.PermittedWickrAwsNetworks {
			n := s.PermittedWickrAwsNetworks[i]
			elems = append(elems, &wickrAwsNetworkModel{
				NetworkID: fwflex.StringToFramework(ctx, n.NetworkId),
				Region:    fwflex.StringToFramework(ctx, n.Region),
			})
		}
		v, d := fwtypes.NewListNestedObjectValueOfSlice(ctx, elems, nil)
		diags.Append(d...)
		m.PermittedWickrAwsNetworks = v
	} else {
		m.PermittedWickrAwsNetworks = fwtypes.NewListNestedObjectValueOfNull[wickrAwsNetworkModel](ctx)
	}
	if len(s.PermittedWickrEnterpriseNetworks) > 0 && !prior.permittedWickrEnterpriseNetworks {
		elems := make([]*wickrEnterpriseNetworkModel, 0, len(s.PermittedWickrEnterpriseNetworks))
		for i := range s.PermittedWickrEnterpriseNetworks {
			n := s.PermittedWickrEnterpriseNetworks[i]
			elems = append(elems, &wickrEnterpriseNetworkModel{
				Domain:    fwflex.StringToFramework(ctx, n.Domain),
				NetworkID: fwflex.StringToFramework(ctx, n.NetworkId),
			})
		}
		v, d := fwtypes.NewListNestedObjectValueOfSlice(ctx, elems, nil)
		diags.Append(d...)
		m.PermittedWickrEnterpriseNetworks = v
	} else {
		m.PermittedWickrEnterpriseNetworks = fwtypes.NewListNestedObjectValueOfNull[wickrEnterpriseNetworkModel](ctx)
	}

	v, d := fwtypes.NewListNestedObjectValueOfPtr(ctx, &m)
	diags.Append(d...)
	return v, diags
}

// flattenStringSliceToListOfString converts a []string to the custom
// `fwtypes.ListValueOf[types.String]` type used in the schema for string-list
// attributes. A nil or empty input produces a null list (matching the
// provider's `FlattenFrameworkStringValueList` convention).
//
// nosemgrep:ci.semgrep.framework.manual-flattener-functions
func flattenStringSliceToListOfString(ctx context.Context, vs []string) fwtypes.ListValueOf[types.String] {
	return fwtypes.ListValueOf[types.String]{ListValue: fwflex.FlattenFrameworkStringValueList(ctx, vs)}
}

// optionalComputedBool returns a `schema.BoolAttribute` declared
// Optional+Computed with `UseStateForUnknown`. Used for every boolean leaf
// on the `settings` nested attribute tree, where AWS populates
// server-side defaults on Create and we want subsequent refreshes to
// carry that value forward without drift.
func optionalComputedBool() schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Bool{
			boolplanmodifier.UseStateForUnknown(),
		},
	}
}

// optionalComputedInt64 is the `types.Int64` counterpart of
// `optionalComputedBool`. Callers can pass extra validators (e.g.,
// `int64validator.OneOf(...)` for the small number of settings leaves
// that have an enum-like allowed set).
func optionalComputedInt64(extraValidators ...validator.Int64) schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
		Validators: extraValidators,
	}
}

// optionalComputedListOfString returns a `schema.ListAttribute` for a
// list-of-string settings leaf, declared Optional+Computed with
// `UseStateForUnknown`.
func optionalComputedListOfString() schema.ListAttribute {
	return schema.ListAttribute{
		CustomType:  fwtypes.ListOfStringType,
		ElementType: types.StringType,
		Optional:    true,
		Computed:    true,
		PlanModifiers: []planmodifier.List{
			listplanmodifier.UseStateForUnknown(),
		},
	}
}

// securityGroupSettingsScalarAttributes returns the scalar nested-attribute
// map for the inside of the `settings` `NestedBlockObject.Attributes`.
// Scalar leaves (bool/int64/list-of-string) are declared as attributes;
// nested list-of-objects (`calling`, `password_requirements`, `shredder`,
// the two permitted-network lists) are declared as nested blocks via
// `securityGroupSettingsNestedBlocks` below.
//
// Every leaf is Optional+Computed with `UseStateForUnknown` so that
// server-populated defaults at Create time flow into state without
// tripping Terraform's post-apply consistency check (see task 6.6
// Failure class A).
func securityGroupSettingsScalarAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"always_reauthenticate":               optionalComputedBool(),
		"atak_package_values":                 optionalComputedListOfString(),
		"check_for_updates":                   optionalComputedBool(),
		"enable_atak":                         optionalComputedBool(),
		"enable_crash_reports":                optionalComputedBool(),
		"enable_file_download":                optionalComputedBool(),
		"enable_guest_federation":             optionalComputedBool(),
		"enable_notification_preview":         optionalComputedBool(),
		"enable_open_access_option":           optionalComputedBool(),
		"enable_restricted_global_federation": optionalComputedBool(),
		"federation_mode":                     optionalComputedInt64(int64validator.OneOf(1, 2)),
		"files_enabled":                       optionalComputedBool(),
		"force_device_lockout":                optionalComputedInt64(),
		"force_open_access":                   optionalComputedBool(),
		"force_read_receipts":                 optionalComputedBool(),
		"global_federation":                   optionalComputedBool(),
		"is_ato_enabled":                      optionalComputedBool(),
		"is_link_preview_enabled":             optionalComputedBool(),
		"location_allow_maps":                 optionalComputedBool(),
		"location_enabled":                    optionalComputedBool(),
		"lockout_threshold":                   optionalComputedInt64(),
		"max_auto_download_size":              optionalComputedInt64(),
		"max_bor":                             optionalComputedInt64(),
		"max_ttl":                             optionalComputedInt64(),
		"message_forwarding_enabled":          optionalComputedBool(),
		"permitted_networks":                  optionalComputedListOfString(),
		"presence_enabled":                    optionalComputedBool(),
		"quick_responses":                     optionalComputedListOfString(),
		"show_master_recovery_key":            optionalComputedBool(),
		"sso_max_idle_minutes":                optionalComputedInt64(),
	}
}

// securityGroupSettingsNestedBlocks returns the nested block map for the
// inside of the `settings` `NestedBlockObject.Blocks`. Each nested
// list-of-one object (`calling`, `password_requirements`, `shredder`) and
// list-of-many object (`permitted_wickr_aws_networks`,
// `permitted_wickr_enterprise_networks`) is a `ListNestedBlock` in the
// v5-compatible schema. Their leaves remain Optional+Computed attributes
// inside each block's `NestedBlockObject.Attributes`.
func securityGroupSettingsNestedBlocks(ctx context.Context) map[string]schema.Block {
	return map[string]schema.Block{
		"calling": schema.ListNestedBlock{
			CustomType: fwtypes.NewListNestedObjectTypeOf[callingSettingsModel](ctx),
			Validators: []validator.List{
				listvalidator.SizeAtMost(1),
			},
			// Tier note: every `calling.*` leaf requires a PREMIUM network
			// plan (see https://aws.amazon.com/wickr/pricing/ → Admin
			// controls → Calling). Setting any of them on a STANDARD
			// network causes `UpdateSecurityGroup` to return HTTP 402
			// ("this feature requires a different level plan"); the
			// provider enriches that error with the offending field list
			// via `enrichPlanTierError`.
			//
			// SDK gap note: the AWS API's JSON response for the `CALLING`
			// (uppercase) key includes fields
			// (`canAddtoCall`, `canStartGroupCall`, `canStartRoomCall`,
			// `canStartScreenShare`) that are absent from the Go SDK's
			// `types.CallingSettings`. See
			// `.kiro/specs/aws-wickr-service/aws-sdk-go-v2-issue.md`. The
			// three leaves we do expose are the ones the SDK can send
			// and receive safely.
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					"can_start_11_call": optionalComputedBool(),
					"can_video_call":    optionalComputedBool(),
					"force_tcp_call":    optionalComputedBool(),
				},
			},
		},
		"password_requirements": schema.ListNestedBlock{
			CustomType: fwtypes.NewListNestedObjectTypeOf[passwordRequirementsModel](ctx),
			Validators: []validator.List{
				// Hard-block password_requirements until the upstream
				// `aws-sdk-go-v2/service/wickr` type coverage for
				// `types.PasswordRequirements` matches the AWS API.
				// The API's Update requires a `regex` field (visible on
				// GetSecurityGroup / UpdateSecurityGroup responses) that
				// the Go SDK struct lacks, so SDK-built payloads are
				// rejected with `"This request contains invalid options
				// or data."` from the API. This is independent of plan
				// tier — `password_requirements` is available on both
				// STANDARD and PREMIUM per
				// https://aws.amazon.com/wickr/pricing/ → Admin
				// controls → Security and compliance.
				//
				// See `.kiro/specs/aws-wickr-service/aws-sdk-go-v2-issue.md`
				// for the full SDK-vs-API gap list.
				listvalidator.SizeAtMost(0),
			},
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					"lowercase":  optionalComputedInt64(),
					"min_length": optionalComputedInt64(),
					"numbers":    optionalComputedInt64(),
					"symbols":    optionalComputedInt64(),
					"uppercase":  optionalComputedInt64(),
				},
			},
		},
		"shredder": schema.ListNestedBlock{
			CustomType: fwtypes.NewListNestedObjectTypeOf[shredderSettingsModel](ctx),
			Validators: []validator.List{
				// Hard-block the shredder sub-block until the upstream
				// `aws-sdk-go-v2/service/wickr` type coverage for
				// `types.ShredderSettings` matches the AWS API. The
				// API requires `canProcessInBackground` (visible on
				// GetSecurityGroup / UpdateSecurityGroup responses)
				// and rejects updates that omit it; the Go SDK struct
				// has no such field, so the SDK cannot produce a
				// request body the API will accept. This is
				// independent of plan tier — `shredder` is available
				// on both STANDARD and PREMIUM per
				// https://aws.amazon.com/wickr/pricing/ → Admin
				// controls → Messaging.
				//
				// See `.kiro/specs/aws-wickr-service/aws-sdk-go-v2-issue.md`
				// for the full SDK-vs-API gap list. Surfacing this as
				// a plan-time error is better than letting Apply fail
				// with a generic "invalid options or data" from AWS.
				listvalidator.SizeAtMost(0),
			},
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					"can_process_manually": optionalComputedBool(),
					// Valid shredder intensities per SDK doc (see
					// `types.ShredderSettings.Intensity`): {0, 20, 60, 100}.
					"intensity": optionalComputedInt64(int64validator.OneOf(0, 20, 60, 100)),
				},
			},
		},
		"permitted_wickr_aws_networks": schema.ListNestedBlock{
			CustomType: fwtypes.NewListNestedObjectTypeOf[wickrAwsNetworkModel](ctx),
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					"network_id":     schema.StringAttribute{Required: true},
					names.AttrRegion: schema.StringAttribute{Required: true},
				},
			},
		},
		"permitted_wickr_enterprise_networks": schema.ListNestedBlock{
			CustomType: fwtypes.NewListNestedObjectTypeOf[wickrEnterpriseNetworkModel](ctx),
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					names.AttrDomain: schema.StringAttribute{Required: true},
					"network_id":     schema.StringAttribute{Required: true},
				},
			},
		},
	}
}

// securityGroupImportID parses the user-supplied composite identifier for
// `terraform import aws_wickr_security_group.<addr> <id>` and maps it onto
// the resource's two parameterized identity attributes (`network_id`,
// `security_group_id`). Joined by the provider-standard `,` separator
// exposed via `intflex.ResourceIdSeparator` (see `docs/id-attributes.md`).
var (
	_ inttypes.ImportIDParser = securityGroupImportID{}
)

type securityGroupImportID struct{}

func (securityGroupImportID) Parse(id string) (string, map[string]any, error) {
	networkID, groupID, found := strings.Cut(id, intflex.ResourceIdSeparator)
	if !found || networkID == "" || groupID == "" {
		return "", nil, fmt.Errorf("id %q should be in the format <network_id>%s<security_group_id>", id, intflex.ResourceIdSeparator)
	}

	return id, map[string]any{
		"network_id":        networkID,
		"security_group_id": groupID,
	}, nil
}

// userSetPremiumOnlyFields returns the subset of AWS Wickr settings
// fields whose tier matrix requires a PREMIUM (or premium-trial)
// network plan that the user explicitly set in the plan's `settings`
// block. A STANDARD network that receives any of these fields in a
// Create or Update request gets rejected by AWS with HTTP 402 and the
// message "this feature requires a different level plan"; this
// function is used to enrich that generic error with an actionable
// list of offending fields.
//
// The list of PREMIUM-only fields is sourced from the canonical
// feature comparison on https://aws.amazon.com/wickr/pricing/ ("Admin
// controls" table); sub-block fields are enumerated with their outer
// block prefix. Maintenance note: when a field moves between tiers
// upstream, update the `setIf` calls below and the matching guidance
// in `website/docs/r/wickr_security_group.html.markdown`.
func userSetPremiumOnlyFields(ctx context.Context, settings fwtypes.ListNestedObjectValueOf[securityGroupSettingsModel]) []string {
	if settings.IsNull() || settings.IsUnknown() {
		return nil
	}
	m, d := settings.ToPtr(ctx)
	if d.HasError() || m == nil {
		return nil
	}
	var set []string
	setIf := func(name string, isConfigured bool) {
		if isConfigured {
			set = append(set, name)
		}
	}
	isConfiguredBool := func(v types.Bool) bool { return !v.IsNull() && !v.IsUnknown() }
	isConfiguredInt64 := func(v types.Int64) bool { return !v.IsNull() && !v.IsUnknown() }
	isConfiguredList := func(v fwtypes.ListValueOf[types.String]) bool {
		return !v.IsNull() && !v.IsUnknown()
	}

	setIf("always_reauthenticate", isConfiguredBool(m.AlwaysReauthenticate))
	setIf("check_for_updates", isConfiguredBool(m.CheckForUpdates))
	setIf("enable_atak", isConfiguredBool(m.EnableAtak))
	setIf("enable_file_download", isConfiguredBool(m.EnableFileDownload))
	setIf("enable_guest_federation", isConfiguredBool(m.EnableGuestFederation))
	setIf("enable_notification_preview", isConfiguredBool(m.EnableNotificationPreview))
	setIf("enable_open_access_option", isConfiguredBool(m.EnableOpenAccessOption))
	setIf("files_enabled", isConfiguredBool(m.FilesEnabled))
	setIf("force_device_lockout", isConfiguredInt64(m.ForceDeviceLockout))
	setIf("force_open_access", isConfiguredBool(m.ForceOpenAccess))
	setIf("force_read_receipts", isConfiguredBool(m.ForceReadReceipts))
	setIf("is_ato_enabled", isConfiguredBool(m.IsAtoEnabled))
	setIf("max_auto_download_size", isConfiguredInt64(m.MaxAutoDownloadSize))
	setIf("max_bor", isConfiguredInt64(m.MaxBor))
	setIf("max_ttl", isConfiguredInt64(m.MaxTtl))
	setIf("show_master_recovery_key", isConfiguredBool(m.ShowMasterRecoveryKey))
	setIf("sso_max_idle_minutes", isConfiguredInt64(m.SsoMaxIdleMinutes))
	setIf("atak_package_values", isConfiguredList(m.AtakPackageValues))

	if !m.Calling.IsNull() && !m.Calling.IsUnknown() {
		if c, cd := m.Calling.ToPtr(ctx); !cd.HasError() && c != nil {
			setIf("calling.can_start_11_call", isConfiguredBool(c.CanStart11Call))
			setIf("calling.can_video_call", isConfiguredBool(c.CanVideoCall))
			setIf("calling.force_tcp_call", isConfiguredBool(c.ForceTcpCall))
		}
	}

	return set
}

// enrichPlanTierError detects the AWS Wickr "feature requires a
// different level plan" (HTTP 402) response and replaces it with a
// more actionable error that names the user-set PREMIUM-only fields
// from the plan. If `err` is any other shape, it's returned
// unchanged.
func enrichPlanTierError(ctx context.Context, err error, settings fwtypes.ListNestedObjectValueOf[securityGroupSettingsModel]) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if !strings.Contains(msg, "different level plan") &&
		!strings.Contains(msg, "StatusCode: 402") &&
		!strings.Contains(msg, "invalid options or data") {
		return err
	}
	offending := userSetPremiumOnlyFields(ctx, settings)
	if len(offending) == 0 {
		// The API flagged a tier violation but we don't see a
		// PREMIUM-only field in the plan. Return the raw error so
		// the user still sees the AWS signal rather than silencing
		// it.
		return err
	}
	return fmt.Errorf(
		"the AWS Wickr network's plan tier does not permit the following settings fields: %s. "+
			"Upgrade the network to the PREMIUM plan (or PREMIUM free trial) via the AWS Wickr "+
			"Management Console, or remove these fields from the `settings` block. See the "+
			"feature-comparison matrix at https://aws.amazon.com/wickr/pricing/. Underlying API error: %w",
		strings.Join(offending, ", "),
		err,
	)
}

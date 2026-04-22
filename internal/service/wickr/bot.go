// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	intflex "github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	inttypes "github.com/hashicorp/terraform-provider-aws/internal/types"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_wickr_bot", name="Bot")
// @IdentityAttribute("network_id")
// @IdentityAttribute("bot_id")
// @ImportIDHandler("botImportID")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/wickr;wickr.GetBotOutput")
// @Testing(importStateIdAttributes="network_id;bot_id", importStateIdAttributesSep="flex.ResourceIdSeparator")
// @Testing(preCheck="testAccPreCheck")
// @Testing(serialize=true)
// @Testing(tagsTest=false)
// @Testing(hasNoPreExistingResource=true)
func newBotResource(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &botResource{}

	// Timeouts per design: Create 30m, Read 10m, Update 30m, Delete 30m.
	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultReadTimeout(10 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(30 * time.Minute)

	return r, nil
}

const (
	ResNameBot = "Bot"
)

type botResource struct {
	framework.ResourceWithModel[botResourceModel]
	framework.WithTimeouts
	framework.WithImportByIdentity
}

func (r *botResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"bot_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// challenge is the bot password, user-supplied on both Create and
			// Update. The SDK allows rotating via UpdateBot. Declared as
			// Required + Sensitive, symmetric with aws_db_instance.password
			// (Requirement 2.3 Group A item 9). NOT WriteOnly, NOT ephemeral.
			// GetBotOutput does not return the password (only HasChallenge),
			// so state-vs-API drift on this field is not possible.
			"challenge": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			names.AttrDisplayName: schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				Required: true,
			},
			"has_challenge": schema.BoolAttribute{
				Computed: true,
			},
			"last_login": schema.StringAttribute{
				Computed: true,
			},
			"network_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pubkey": schema.StringAttribute{
				Computed: true,
			},
			names.AttrStatus: schema.Int64Attribute{
				Computed: true,
			},
			"suspend": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"suspended": schema.BoolAttribute{
				Computed: true,
			},
			"uname": schema.StringAttribute{
				Computed: true,
			},
			// Implementation-time verification for open question #7:
			// The Wickr API enforces that bot usernames must end in "bot"
			// (per types.User SDK doc). Additional validation rules beyond
			// the suffix constraint should be probed during acceptance
			// testing and encoded here. The regex below enforces the
			// documented suffix rule; add further character-class or
			// length constraints if the live API rejects inputs that pass
			// this regex.
			names.AttrUsername: schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexache.MustCompile(`bot$`),
						"username must end in \"bot\"",
					),
					stringvalidator.LengthAtLeast(4),
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

func (r *botResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan botResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := plan.NetworkID.ValueString()
	username := plan.Username.ValueString()

	input := wickr.CreateBotInput{
		Challenge: plan.Challenge.ValueStringPointer(),
		GroupId:   plan.GroupID.ValueStringPointer(),
		NetworkId: aws.String(networkID),
		Username:  aws.String(username),
	}
	if !plan.DisplayName.IsNull() && !plan.DisplayName.IsUnknown() {
		input.DisplayName = plan.DisplayName.ValueStringPointer()
	}

	out, err := conn.CreateBot(ctx, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, username))
		return
	}
	if out == nil || out.BotId == nil {
		smerr.AddError(ctx, &resp.Diagnostics, tfresource.NewEmptyResultError(), smerr.ID, compositeID(networkID, username))
		return
	}

	botID := aws.ToString(out.BotId)
	plan.BotID = fwflex.StringToFramework(ctx, out.BotId)

	// CreateBotOutput does not return the full attribute set; follow up
	// with GetBot to populate all Computed fields.
	bot, err := findBotByID(ctx, conn, networkID, botID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, botID))
		return
	}

	// If the user set suspend = true, apply it via UpdateBot since
	// CreateBotInput does not have a Suspend field.
	if !plan.Suspend.IsNull() && !plan.Suspend.IsUnknown() && plan.Suspend.ValueBool() {
		updateInput := wickr.UpdateBotInput{
			BotId:     aws.String(botID),
			NetworkId: aws.String(networkID),
			Suspend:   aws.Bool(true),
		}
		_, err := conn.UpdateBot(ctx, &updateInput)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, botID))
			return
		}
		// Re-read after the suspend update.
		bot, err = findBotByID(ctx, conn, networkID, botID)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, botID))
			return
		}
	}

	// Preserve the user-supplied challenge from the plan because GetBot
	// does not return the password (only HasChallenge).
	challenge := plan.Challenge
	// Preserve the user-supplied suspend from the plan. The API's
	// Suspended field may lag behind the desired state.
	// When suspend is not set in config (null/unknown), use the API value.
	suspend := plan.Suspend

	flattenBot(ctx, bot, &plan)

	plan.Challenge = challenge
	if !suspend.IsNull() && !suspend.IsUnknown() {
		plan.Suspend = suspend
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *botResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state botResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := state.NetworkID.ValueString()
	botID := state.BotID.ValueString()

	out, err := findBotByID(ctx, conn, networkID, botID)
	if retry.NotFound(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	// Mirror the Network/SecurityGroup pattern: treat ForbiddenError and
	// orphaned-child deserialization failures as "gone" so terraform
	// refresh after an out-of-band parent-network delete cleanly removes
	// the resource from state.
	if isBotOrphanedChildError(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, botID))
		return
	}

	// Preserve the user-supplied challenge from prior state because
	// GetBot does not return the password. On import, prior state is
	// empty, so challenge will be null — that's correct because the
	// user must supply it in config for subsequent applies.
	challenge := state.Challenge
	// Preserve the user-supplied suspend from prior state. The API's
	// Suspended field may lag behind the desired state due to eventual
	// consistency. On import, suspend will be null.
	suspend := state.Suspend

	flattenBot(ctx, out, &state)

	state.Challenge = challenge
	if !suspend.IsNull() && !suspend.IsUnknown() {
		state.Suspend = suspend
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &state))
}

func (r *botResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state botResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := state.NetworkID.ValueString()
	botID := state.BotID.ValueString()

	// Build UpdateBotInput. The Wickr API's UpdateBot requires BotId and
	// NetworkId; all other fields are optional. We always send GroupId
	// (Required in schema) to avoid validation errors.
	input := wickr.UpdateBotInput{
		BotId:     aws.String(botID),
		NetworkId: aws.String(networkID),
		GroupId:   plan.GroupID.ValueStringPointer(),
	}

	needsUpdate := false

	// challenge is always sent on Update when it changes (password rotation).
	if !plan.Challenge.Equal(state.Challenge) {
		input.Challenge = plan.Challenge.ValueStringPointer()
		needsUpdate = true
	}
	if !plan.DisplayName.Equal(state.DisplayName) {
		input.DisplayName = plan.DisplayName.ValueStringPointer()
		needsUpdate = true
	}
	if !plan.GroupID.Equal(state.GroupID) {
		needsUpdate = true
	}
	if !plan.Suspend.Equal(state.Suspend) {
		input.Suspend = plan.Suspend.ValueBoolPointer()
		needsUpdate = true
	}

	if needsUpdate {
		_, err := conn.UpdateBot(ctx, &input)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, botID))
			return
		}
	}

	// Re-read to populate all Computed fields.
	out, err := findBotByID(ctx, conn, networkID, botID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, botID))
		return
	}

	// Preserve the user-supplied challenge from the plan.
	challenge := plan.Challenge
	// Preserve the user-supplied suspend from the plan because the API
	// may not immediately reflect the suspend state change.
	suspend := plan.Suspend

	flattenBot(ctx, out, &plan)

	plan.Challenge = challenge
	if !suspend.IsNull() && !suspend.IsUnknown() {
		plan.Suspend = suspend
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *botResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state botResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := state.NetworkID.ValueString()
	botID := state.BotID.ValueString()

	input := wickr.DeleteBotInput{
		BotId:     aws.String(botID),
		NetworkId: aws.String(networkID),
	}
	_, err := conn.DeleteBot(ctx, &input)
	// Treat already-gone responses as success: ResourceNotFoundError,
	// ForbiddenError (parent network deleting), orphaned-child
	// deserialization failures, or "Bot not found" (HTTP 404).
	if errs.IsA[*awstypes.ResourceNotFoundError](err) || isBotOrphanedChildError(err) || errs.Contains(err, "Bot not found") {
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, botID))
		return
	}
}

// isBotOrphanedChildError returns true when err indicates the bot (or its
// parent network) is gone. Mirrors isSecurityGroupOrphanedChildError.
func isBotOrphanedChildError(err error) bool {
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

// flattenBot populates the model from a GetBotOutput using AutoFlex for
// the bulk of the work. Fields that GetBot does not return (challenge,
// network_id) must be preserved by the caller. The `suspend` attribute
// (the user-facing input toggle) is set from `Suspended` (the API output)
// after AutoFlex runs, since AutoFlex only maps by exact name.
// nosemgrep:ci.semgrep.framework.manual-flattener-functions
func flattenBot(ctx context.Context, out *wickr.GetBotOutput, m *botResourceModel) {
	fwflex.Flatten(ctx, out, m)

	// `suspend` is the user-facing input argument that maps to
	// `UpdateBotInput.Suspend`; `suspended` is the read-only output from
	// `GetBotOutput.Suspended`. AutoFlex maps `Suspended` → `suspended`
	// by name but cannot map it to `suspend` as well. Sync them here.
	m.Suspend = m.Suspended
}

// botResourceModel mirrors the Framework-resource schema.
type botResourceModel struct {
	framework.WithRegionModel
	BotID        types.String   `tfsdk:"bot_id"`
	Challenge    types.String   `tfsdk:"challenge"`
	DisplayName  types.String   `tfsdk:"display_name"`
	GroupID      types.String   `tfsdk:"group_id"`
	HasChallenge types.Bool     `tfsdk:"has_challenge"`
	LastLogin    types.String   `tfsdk:"last_login"`
	NetworkID    types.String   `tfsdk:"network_id"`
	Pubkey       types.String   `tfsdk:"pubkey"`
	Status       types.Int64    `tfsdk:"status"`
	Suspend      types.Bool     `tfsdk:"suspend"`
	Suspended    types.Bool     `tfsdk:"suspended"`
	Timeouts     timeouts.Value `tfsdk:"timeouts"`
	Uname        types.String   `tfsdk:"uname"`
	Username     types.String   `tfsdk:"username"`
}

// botImportID parses the compound import ID "network_id,bot_id".
var _ inttypes.ImportIDParser = botImportID{}

type botImportID struct{}

func (botImportID) Parse(id string) (string, map[string]any, error) {
	networkID, botID, found := strings.Cut(id, intflex.ResourceIdSeparator)
	if !found || networkID == "" || botID == "" {
		return "", nil, fmt.Errorf("id %q should be in the format <network_id>%s<bot_id>", id, intflex.ResourceIdSeparator)
	}

	result := map[string]any{
		"network_id": networkID,
		"bot_id":     botID,
	}

	return id, result, nil
}

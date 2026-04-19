// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_wickr_network", name="Network")
// @ArnIdentity
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/wickr;wickr.GetNetworkOutput")
// @Testing(preCheck="testAccPreCheck")
// @Testing(serialize=true)
// @Testing(tagsTest=false)
// @Testing(hasNoPreExistingResource=true)
func newNetworkResource(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &networkResource{}

	// Timeouts per design: Create 30m, Read 10m, Update 30m, Delete 60m.
	// Delete is doubled vs default because DeleteNetwork cascades through
	// child resources (users, bots, security groups, settings) and may take time.
	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultReadTimeout(10 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(60 * time.Minute)

	return r, nil
}

const (
	ResNameNetwork = "Network"
)

// Implementation note for open question #3 (retry-error discrimination):
// The AWS SDK for Go v2 default retryer (enabled by the provider-level
// `conns.AWSClient` wrapper) already retries `*awstypes.RateLimitError` and
// `*awstypes.InternalServerError`. No custom retryer is wired into
// `service_package.go` for Wickr: the default behavior is sufficient for
// the control-plane operations used by this resource. If that changes, add
// a `withExtraOptions` hook alongside `NewClient` and cite the motivating
// retryable error type here.
type networkResource struct {
	framework.ResourceWithModel[networkResourceModel]
	framework.WithTimeouts
	framework.WithImportByIdentity
}

func (r *networkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"access_level": schema.StringAttribute{
				CustomType: fwtypes.StringEnumType[awstypes.AccessLevel](),
				Required:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			names.AttrARN: framework.ARNAttributeComputedOnly(),
			names.AttrAWSAccountID: schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enable_premium_free_trial": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			// Open question (deferred): `encryption_key_arn` is documented on
			// the `CreateNetwork`, `UpdateNetwork`, and `GetNetwork` API
			// contracts (see
			// https://docs.aws.amazon.com/wickr/latest/APIReference/API_Network.html)
			// and modeled in `aws-sdk-go-v2/service/wickr` as
			// `*string`, but the live service (verified 2026-04-19, us-east-1,
			// both STANDARD and PREMIUM tiers) silently accepts the value on
			// Create/Update and never returns it on any subsequent Get/List
			// call. The field is also absent from the AWS Wickr admin console's
			// network-creation UI, suggesting the feature is either pre-announced
			// or not yet rolled out to public customers. The attribute is
			// intentionally omitted from this schema until the service implements
			// end-to-end persistence. Reintroduce it as a backward-compatible
			// minor-version addition when live Get returns the value.
			// Open question #2 (FreeTrialExpiration string format):
			// The SDK doc describes this as "The expiration date and time" but
			// does not specify a format. Surface it as a plain Framework string
			// until a live API call confirms RFC3339, at which point this can be
			// switched to `timetypes.RFC3339Type`. Whoever verifies should
			// update this comment with the observed format and the date tested.
			"free_trial_expiration": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"migration_state": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"network_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"network_name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 20),
				},
			},
			"standing": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
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

func (r *networkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)

	networkName := fwflex.StringValueFromFramework(ctx, plan.NetworkName)
	var input wickr.CreateNetworkInput
	smerr.AddEnrich(ctx, &resp.Diagnostics, fwflex.Expand(ctx, plan, &input))
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := conn.CreateNetwork(ctx, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkName)
		return
	}
	if out == nil || out.NetworkId == nil {
		smerr.AddError(ctx, &resp.Diagnostics, tfresource.NewEmptyResultError(), smerr.ID, networkName)
		return
	}

	networkID := aws.ToString(out.NetworkId)
	plan.NetworkId = fwflex.StringToFramework(ctx, out.NetworkId)

	// CreateNetworkOutput does not include the ARN — follow up with GetNetwork
	// to capture the full attribute set (ARN, AwsAccountId, FreeTrialExpiration, etc.).
	net, err := findNetworkByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Preserve the Create-only `enable_premium_free_trial` value from the
	// plan, because `GetNetworkOutput` does not echo it back and Flatten
	// would otherwise leave the Computed attribute as Unknown.
	enableFreeTrial := plan.EnablePremiumFreeTrial

	smerr.AddEnrich(ctx, &resp.Diagnostics, fwflex.Flatten(ctx, net, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	if enableFreeTrial.IsNull() || enableFreeTrial.IsUnknown() {
		plan.EnablePremiumFreeTrial = types.BoolValue(false)
	} else {
		plan.EnablePremiumFreeTrial = enableFreeTrial
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *networkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)

	// Import only populates the identity (ARN). Derive network_id from the
	// ARN's last path segment when state doesn't already have it. The ARN
	// shape per the AWS Wickr API Reference is
	//   arn:aws:wickr:<region>:<account>:network/<networkId>
	// so the 8-digit id is always the final `/`-separated segment.
	networkID := fwflex.StringValueFromFramework(ctx, state.NetworkId)
	if networkID == "" {
		if arn := fwflex.StringValueFromFramework(ctx, state.NetworkArn); arn != "" {
			if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx+1 < len(arn) {
				networkID = arn[idx+1:]
			}
		}
	}
	if networkID == "" {
		smerr.AddError(ctx, &resp.Diagnostics, errors.New("network_id could not be determined from state or ARN"))
		return
	}

	out, err := findNetworkByID(ctx, conn, networkID)
	if retry.NotFound(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	// Delete-in-progress or already-deleted networks return
	// *awstypes.ForbiddenError rather than ResourceNotFoundError; treat that
	// as gone so `terraform refresh` after an out-of-band delete cleanly
	// removes the resource from state rather than erroring out.
	if errs.IsA[*awstypes.ForbiddenError](err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Preserve the Create-only `enable_premium_free_trial` from prior state
	// (captured before Flatten clobbers it); GetNetworkOutput doesn't echo it.
	// On import, prior state is empty, so fall back to `false` to match the
	// schema's Optional+Computed semantics.
	enableFreeTrial := state.EnablePremiumFreeTrial

	smerr.AddEnrich(ctx, &resp.Diagnostics, fwflex.Flatten(ctx, out, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	if enableFreeTrial.IsNull() || enableFreeTrial.IsUnknown() {
		state.EnablePremiumFreeTrial = types.BoolValue(false)
	} else {
		state.EnablePremiumFreeTrial = enableFreeTrial
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &state))
}

func (r *networkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state networkResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := fwflex.StringValueFromFramework(ctx, state.NetworkId)

	// UpdateNetwork's only meaningfully mutable field we currently surface
	// is `NetworkName`. `AccessLevel` and `EnablePremiumFreeTrial` carry
	// `RequiresReplace` plan modifiers, so they never reach Update. The SDK
	// exposes `EncryptionKeyArn` on UpdateNetworkInput but the service does
	// not persist it (see schema comment), so we do not wire it.
	if !plan.NetworkName.Equal(state.NetworkName) {
		input := wickr.UpdateNetworkInput{
			NetworkId:   aws.String(networkID),
			NetworkName: plan.NetworkName.ValueStringPointer(),
		}

		_, err := conn.UpdateNetwork(ctx, &input)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
			return
		}
	}

	// UpdateNetwork does not return the updated object; re-read via GetNetwork
	// to populate any computed attributes that the API may have adjusted.
	out, err := findNetworkByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Preserve the Create-only `enable_premium_free_trial` from state because
	// `GetNetworkOutput` does not echo it back (same reason as in Create).
	enableFreeTrial := state.EnablePremiumFreeTrial

	smerr.AddEnrich(ctx, &resp.Diagnostics, fwflex.Flatten(ctx, out, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	plan.EnablePremiumFreeTrial = enableFreeTrial

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *networkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := fwflex.StringValueFromFramework(ctx, state.NetworkId)

	input := wickr.DeleteNetworkInput{
		NetworkId: aws.String(networkID),
	}
	_, err := conn.DeleteNetwork(ctx, &input)
	if errs.IsA[*awstypes.ResourceNotFoundError](err) || errs.IsA[*awstypes.ForbiddenError](err) {
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// DeleteNetwork lifecycle / pending states (verified 2026-04-19, us-east-1):
	// - DeleteNetwork returns HTTP 200 with "Network deletion initiated
	//   successfully" immediately. The network persists briefly afterward.
	// - During that window, GetNetwork returns *awstypes.ForbiddenError
	//   (not ResourceNotFoundError). Treating only ResourceNotFoundError as
	//   "gone" causes the waiter to stall until timeout. We accept both here.
	// - ListNetworks filters the network out as soon as deletion starts, so
	//   customers see it disappear via ListNetworks even before GetNetwork
	//   starts returning a terminal error.
	//
	// Open question #8 (per-account network quota):
	// Per-account quota, if any, has not been observed in this account. If
	// acceptance-test parallelism becomes a problem, add a PreCheck or a
	// serialized TestMain.
	deleteTimeout := r.DeleteTimeout(ctx, state.Timeouts)
	_, err = tfresource.RetryUntilEqual(ctx, deleteTimeout, true, func(ctx context.Context) (bool, error) {
		_, findErr := findNetworkByID(ctx, conn, networkID)
		if retry.NotFound(findErr) {
			return true, nil
		}
		if errs.IsA[*awstypes.ForbiddenError](findErr) {
			return true, nil
		}
		if findErr != nil {
			return false, findErr
		}
		return false, nil
	})
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}
}

// networkResourceModel mirrors the Framework-resource schema. Field names
// match the SDK's `wickr.GetNetworkOutput` (and `types.Network`) exactly so
// AutoFlex's `fwflex.Flatten` can map API fields → Framework attributes by
// name. The exceptions:
//   - `EnablePremiumFreeTrial` has no counterpart in `GetNetworkOutput` (it
//     is Create-only input). We preserve the plan value manually in Create
//     after Flatten so the Computed attribute never resolves to Unknown.
//   - `Timeouts` is Framework-native plumbing, not an API field.
//   - `EncryptionKeyArn` is intentionally omitted (see schema comment) until
//     the AWS Wickr service implements end-to-end persistence of the value.
type networkResourceModel struct {
	framework.WithRegionModel
	AccessLevel            fwtypes.StringEnum[awstypes.AccessLevel] `tfsdk:"access_level"`
	AwsAccountId           types.String                             `tfsdk:"aws_account_id"`
	EnablePremiumFreeTrial types.Bool                               `tfsdk:"enable_premium_free_trial"`
	FreeTrialExpiration    types.String                             `tfsdk:"free_trial_expiration"`
	MigrationState         types.Int64                              `tfsdk:"migration_state"`
	NetworkArn             types.String                             `tfsdk:"arn"`
	NetworkId              types.String                             `tfsdk:"network_id"`
	NetworkName            types.String                             `tfsdk:"network_name"`
	Standing               types.Int64                              `tfsdk:"standing"`
	Timeouts               timeouts.Value                           `tfsdk:"timeouts"`
}

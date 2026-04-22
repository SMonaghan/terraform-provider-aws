// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_wickr_network_settings", name="Network Settings")
// @IdentityAttribute("network_id")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/wickr;wickr.GetNetworkSettingsOutput")
// @Testing(preCheck="testAccPreCheck")
// @Testing(serialize=true)
// @Testing(tagsTest=false)
// @Testing(hasNoPreExistingResource=true)
func newNetworkSettingsResource(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &networkSettingsResource{}

	r.SetDefaultCreateTimeout(10 * time.Minute)
	r.SetDefaultReadTimeout(5 * time.Minute)
	r.SetDefaultUpdateTimeout(10 * time.Minute)
	r.SetDefaultDeleteTimeout(1 * time.Minute)

	return r, nil
}

const (
	ResNameNetworkSettings = "Network Settings"
)

type networkSettingsResource struct {
	framework.ResourceWithModel[networkSettingsResourceModel]
	framework.WithTimeouts
	framework.WithNoOpDelete
	framework.WithImportByIdentity
}

func (r *networkSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"network_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"data_retention": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"enable_client_metrics": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"enable_trusted_data_format": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"read_receipt_config": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[readReceiptConfigModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						names.AttrStatus: schema.StringAttribute{
							CustomType: fwtypes.StringEnumType[awstypes.Status](),
							Optional:   true,
							Computed:   true,
						},
					},
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

func (r *networkSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkSettingsResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := fwflex.StringValueFromFramework(ctx, plan.NetworkID)

	// Singleton-child Create = read-then-update.
	// Verify the parent network exists by reading current settings.
	_, err := findNetworkSettingsByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	// Only call UpdateNetworkSettings if the user configured at least one
	// setting. An empty settings struct causes BadRequestError.
	settings := expandNetworkSettings(ctx, plan.networkSettingsModel)
	if settings != nil {
		input := wickr.UpdateNetworkSettingsInput{
			NetworkId: aws.String(networkID),
			Settings:  settings,
		}

		_, err = conn.UpdateNetworkSettings(ctx, &input)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
			return
		}
	}

	out, err := findNetworkSettingsByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	flattenNetworkSettingsOutput(ctx, out, &plan.networkSettingsModel)

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *networkSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkSettingsResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := fwflex.StringValueFromFramework(ctx, state.NetworkID)

	out, err := findNetworkSettingsByID(ctx, conn, networkID)
	if retry.NotFound(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	if errs.IsA[*awstypes.ForbiddenError](err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	flattenNetworkSettingsOutput(ctx, out, &state.networkSettingsModel)

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &state))
}

func (r *networkSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state networkSettingsResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().WickrClient(ctx)
	networkID := fwflex.StringValueFromFramework(ctx, state.NetworkID)

	// Read current settings to use as base for the full payload.
	input := wickr.UpdateNetworkSettingsInput{
		NetworkId: aws.String(networkID),
		Settings:  expandNetworkSettings(ctx, plan.networkSettingsModel),
	}

	_, err := conn.UpdateNetworkSettings(ctx, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	out, err := findNetworkSettingsByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	flattenNetworkSettingsOutput(ctx, out, &plan.networkSettingsModel)

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &plan))
}

func (r *networkSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("network_id"), req, resp)
}

type networkSettingsResourceModel struct {
	networkSettingsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// networkSettingsModel contains the fields shared between the resource and
// data source models. flattenNetworkSettingsOutput populates this struct
// from the flat []types.Setting list returned by GetNetworkSettings.
type networkSettingsModel struct {
	framework.WithRegionModel
	NetworkID               types.String                                            `tfsdk:"network_id"`
	DataRetention           types.Bool                                              `tfsdk:"data_retention"`
	EnableClientMetrics     types.Bool                                              `tfsdk:"enable_client_metrics"`
	EnableTrustedDataFormat types.Bool                                              `tfsdk:"enable_trusted_data_format"`
	ReadReceiptConfig       fwtypes.ListNestedObjectValueOf[readReceiptConfigModel] `tfsdk:"read_receipt_config"`
}

type readReceiptConfigModel struct {
	Status fwtypes.StringEnum[awstypes.Status] `tfsdk:"status"`
}

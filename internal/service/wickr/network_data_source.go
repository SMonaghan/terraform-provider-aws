// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"

	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkDataSource("aws_wickr_network", name="Network")
func newNetworkDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &networkDataSource{}, nil
}

const (
	DSNameNetwork = "Network"
)

type networkDataSource struct {
	framework.DataSourceWithModel[networkDataSourceModel]
}

func (d *networkDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"access_level": schema.StringAttribute{
				CustomType: fwtypes.StringEnumType[awstypes.AccessLevel](),
				Computed:   true,
			},
			names.AttrARN: framework.ARNAttributeComputedOnly(),
			names.AttrAWSAccountID: schema.StringAttribute{
				Computed: true,
			},
			// `GetNetworkOutput` does not expose `EnablePremiumFreeTrial`; the
			// field will flatten to null. That is the expected behavior for a
			// data source reading an existing network — the resource preserves
			// the Create-time value in its own state, but a data source has no
			// prior state to preserve from.
			"enable_premium_free_trial": schema.BoolAttribute{
				Computed: true,
			},
			"free_trial_expiration": schema.StringAttribute{
				Computed: true,
			},
			"migration_state": schema.Int64Attribute{
				Computed: true,
			},
			"network_id": schema.StringAttribute{
				Required: true,
			},
			"network_name": schema.StringAttribute{
				Computed: true,
			},
			"standing": schema.Int64Attribute{
				Computed: true,
			},
		},
	}
}

func (d *networkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data networkDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn := d.Meta().WickrClient(ctx)

	networkID := data.NetworkId.ValueString()

	out, err := findNetworkByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	resp.Diagnostics.Append(fwflex.Flatten(ctx, out, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// networkDataSourceModel mirrors `networkResourceModel` (see network.go),
// minus the `Timeouts` field (data sources have no timeouts) and with every
// field Computed except `NetworkId` (the Required lookup argument). Field
// names and `tfsdk:` tags match the resource model so `fwflex.Flatten(ctx,
// out, &data)` maps `GetNetworkOutput` → Framework attributes by name.
//
// `EnablePremiumFreeTrial` has no counterpart in `GetNetworkOutput` and will
// flatten to null — see the schema comment above.
// `EncryptionKeyArn` is intentionally omitted: the live service accepts the
// value on Create/Update but never echoes it on Get, so the resource omits
// it as well.
type networkDataSourceModel struct {
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
}

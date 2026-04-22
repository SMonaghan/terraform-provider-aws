// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"

	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkDataSource("aws_wickr_network_settings", name="Network Settings")
func newNetworkSettingsDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &networkSettingsDataSource{}, nil
}

const (
	DSNameNetworkSettings = "Network Settings"
)

type networkSettingsDataSource struct {
	framework.DataSourceWithModel[networkSettingsDataSourceModel]
}

func (d *networkSettingsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"network_id": schema.StringAttribute{
				Required: true,
			},
			"data_retention": schema.BoolAttribute{
				Computed: true,
			},
			"enable_client_metrics": schema.BoolAttribute{
				Computed: true,
			},
			"enable_trusted_data_format": schema.BoolAttribute{
				Computed: true,
			},
		},
		Blocks: map[string]schema.Block{
			"read_receipt_config": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[readReceiptConfigModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						names.AttrStatus: schema.StringAttribute{
							CustomType: fwtypes.StringEnumType[awstypes.Status](),
							Computed:   true,
						},
					},
				},
			},
		},
	}
}

func (d *networkSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data networkSettingsDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := d.Meta().WickrClient(ctx)
	networkID := fwflex.StringValueFromFramework(ctx, data.NetworkID)

	out, err := findNetworkSettingsByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	flattenNetworkSettingsOutput(ctx, out, &data.networkSettingsModel)

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data))
}

// networkSettingsDataSourceModel mirrors the resource model minus the
// Timeouts field (data sources have no timeouts). It embeds the shared
// networkSettingsModel so the same flattenNetworkSettingsOutput function
// can populate both the resource and data source models.
type networkSettingsDataSourceModel struct {
	networkSettingsModel
}

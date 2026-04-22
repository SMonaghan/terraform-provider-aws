// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkDataSource("aws_wickr_bot", name="Bot")
func newBotDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &botDataSource{}, nil
}

const (
	DSNameBot = "Bot"
)

type botDataSource struct {
	framework.DataSourceWithModel[botDataSourceModel]
}

func (d *botDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"bot_id": schema.StringAttribute{
				Required: true,
			},
			names.AttrDisplayName: schema.StringAttribute{
				Computed: true,
			},
			"group_id": schema.StringAttribute{
				Computed: true,
			},
			"has_challenge": schema.BoolAttribute{
				Computed: true,
			},
			"last_login": schema.StringAttribute{
				Computed: true,
			},
			"network_id": schema.StringAttribute{
				Required: true,
			},
			"pubkey": schema.StringAttribute{
				Computed: true,
			},
			names.AttrStatus: schema.Int64Attribute{
				Computed: true,
			},
			"suspended": schema.BoolAttribute{
				Computed: true,
			},
			"uname": schema.StringAttribute{
				Computed: true,
			},
			names.AttrUsername: schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *botDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data botDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := d.Meta().WickrClient(ctx)

	networkID := data.NetworkID.ValueString()
	botID := data.BotID.ValueString()

	out, err := findBotByID(ctx, conn, networkID, botID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, botID))
		return
	}

	resp.Diagnostics.Append(fwflex.Flatten(ctx, out, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Restore the Required lookup keys that AutoFlex may have overwritten.
	data.NetworkID = types.StringValue(networkID)
	data.BotID = types.StringValue(botID)

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data))
}

// botDataSourceModel mirrors the resource's Computed attributes minus
// `challenge` (GetBot does not return the password; Requirement 12.4),
// `suspend` (user-facing input toggle, not relevant for a read-only data
// source), and `timeouts` (data sources have no timeouts block).
type botDataSourceModel struct {
	framework.WithRegionModel
	BotID        types.String `tfsdk:"bot_id"`
	DisplayName  types.String `tfsdk:"display_name"`
	GroupID      types.String `tfsdk:"group_id"`
	HasChallenge types.Bool   `tfsdk:"has_challenge"`
	LastLogin    types.String `tfsdk:"last_login"`
	NetworkID    types.String `tfsdk:"network_id"`
	Pubkey       types.String `tfsdk:"pubkey"`
	Status       types.Int64  `tfsdk:"status"`
	Suspended    types.Bool   `tfsdk:"suspended"`
	Uname        types.String `tfsdk:"uname"`
	Username     types.String `tfsdk:"username"`
}

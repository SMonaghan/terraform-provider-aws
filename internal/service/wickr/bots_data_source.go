// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/wickr"
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

// @FrameworkDataSource("aws_wickr_bots", name="Bots")
func newBotsDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &botsDataSource{}, nil
}

const (
	DSNameBots = "Bots"
)

type botsDataSource struct {
	framework.DataSourceWithModel[botsDataSourceModel]
}

func (d *botsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"network_id": schema.StringAttribute{
				Required: true,
			},
			names.AttrDisplayName: schema.StringAttribute{
				Optional: true,
			},
			"group_id": schema.StringAttribute{
				Optional: true,
			},
			names.AttrStatus: schema.Int64Attribute{
				Optional: true,
			},
			names.AttrUsername: schema.StringAttribute{
				Optional: true,
			},
			"bots": schema.ListAttribute{
				Computed:   true,
				CustomType: fwtypes.NewListNestedObjectTypeOf[botSummaryModel](ctx),
			},
		},
	}
}

func (d *botsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data botsDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := d.Meta().WickrClient(ctx)

	networkID := data.NetworkID.ValueString()

	input := wickr.ListBotsInput{
		NetworkId: &networkID,
	}

	// Thread optional filters through to ListBotsInput.
	if !data.DisplayName.IsNull() && !data.DisplayName.IsUnknown() {
		input.DisplayName = data.DisplayName.ValueStringPointer()
	}
	if !data.GroupID.IsNull() && !data.GroupID.IsUnknown() {
		input.GroupId = data.GroupID.ValueStringPointer()
	}
	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		input.Status = awstypes.BotStatus(data.Status.ValueInt64())
	}
	if !data.Username.IsNull() && !data.Username.IsUnknown() {
		input.Username = data.Username.ValueStringPointer()
	}

	// Paginate via NewListBotsPaginator and accumulate page.Bots across
	// all pages. An empty result is NOT an error (Requirement 13.5): it
	// simply produces an empty `bots` list.
	var bots []awstypes.Bot
	paginator := wickr.NewListBotsPaginator(conn, &input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err)
			return
		}
		bots = append(bots, page.Bots...)
	}

	// Flatten each bot into the summary model.
	summaries := make([]*botSummaryModel, 0, len(bots))
	for _, bot := range bots {
		m := &botSummaryModel{}
		m.BotID = fwflex.StringToFramework(ctx, bot.BotId)
		m.DisplayName = fwflex.StringToFramework(ctx, bot.DisplayName)
		m.GroupID = fwflex.StringToFramework(ctx, bot.GroupId)
		m.HasChallenge = fwflex.BoolToFramework(ctx, bot.HasChallenge)
		m.LastLogin = fwflex.StringToFramework(ctx, bot.LastLogin)
		m.Pubkey = fwflex.StringToFramework(ctx, bot.Pubkey)
		m.Status = types.Int64Value(int64(bot.Status))
		m.Suspended = fwflex.BoolToFramework(ctx, bot.Suspended)
		m.Uname = fwflex.StringToFramework(ctx, bot.Uname)
		m.Username = fwflex.StringToFramework(ctx, bot.Username)
		summaries = append(summaries, m)
	}

	listVal, listDiags := fwtypes.NewListNestedObjectValueOfSlice(ctx, summaries, nil)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Bots = listVal

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data))
}

// botsDataSourceModel is the top-level schema model. `network_id` is the
// Required filter; `display_name`, `group_id`, `status`, and `username`
// are Optional filters threaded through to `ListBotsInput`; `bots` is a
// Computed list of nested `botSummaryModel` objects.
type botsDataSourceModel struct {
	framework.WithRegionModel
	NetworkID   types.String                                     `tfsdk:"network_id"`
	DisplayName types.String                                     `tfsdk:"display_name"`
	GroupID     types.String                                     `tfsdk:"group_id"`
	Status      types.Int64                                      `tfsdk:"status"`
	Username    types.String                                     `tfsdk:"username"`
	Bots        fwtypes.ListNestedObjectValueOf[botSummaryModel] `tfsdk:"bots"`
}

// botSummaryModel mirrors `awstypes.Bot` (see
// github.com/aws/aws-sdk-go-v2/service/wickr/types). Projections of
// `types.Bot`: `bot_id`, `display_name`, `group_id`, `has_challenge`,
// `last_login`, `pubkey`, `status`, `suspended`, `uname`, `username`.
// **No `challenge`** — GetBot/ListBots does not return it.
type botSummaryModel struct {
	BotID        types.String `tfsdk:"bot_id"`
	DisplayName  types.String `tfsdk:"display_name"`
	GroupID      types.String `tfsdk:"group_id"`
	HasChallenge types.Bool   `tfsdk:"has_challenge"`
	LastLogin    types.String `tfsdk:"last_login"`
	Pubkey       types.String `tfsdk:"pubkey"`
	Status       types.Int64  `tfsdk:"status"`
	Suspended    types.Bool   `tfsdk:"suspended"`
	Uname        types.String `tfsdk:"uname"`
	Username     types.String `tfsdk:"username"`
}

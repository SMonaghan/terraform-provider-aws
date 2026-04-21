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

// @FrameworkDataSource("aws_wickr_security_groups", name="Security Groups")
func newSecurityGroupsDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &securityGroupsDataSource{}, nil
}

const (
	DSNameSecurityGroups = "Security Groups"
)

type securityGroupsDataSource struct {
	framework.DataSourceWithModel[securityGroupsDataSourceModel]
}

func (d *securityGroupsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"network_id": schema.StringAttribute{
				Required: true,
			},
			names.AttrSecurityGroups: schema.ListAttribute{
				Computed:   true,
				CustomType: fwtypes.NewListNestedObjectTypeOf[securityGroupSummaryModel](ctx),
			},
		},
	}
}

func (d *securityGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data securityGroupsDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := d.Meta().WickrClient(ctx)

	networkID := data.NetworkID.ValueString()

	// Paginate via NewListSecurityGroupsPaginator and accumulate
	// page.SecurityGroups across all pages. An empty result is NOT an
	// error (Requirement 8.5): it simply produces an empty
	// `security_groups` list.
	var securityGroups []awstypes.SecurityGroup
	paginator := wickr.NewListSecurityGroupsPaginator(conn, &wickr.ListSecurityGroupsInput{
		NetworkId: &networkID,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err)
			return
		}
		securityGroups = append(securityGroups, page.SecurityGroups...)
	}

	// Flatten each security group into the summary model, including the
	// full settings subtree.
	summaries := make([]*securityGroupSummaryModel, 0, len(securityGroups))
	for _, sg := range securityGroups {
		m := &securityGroupSummaryModel{}
		m.ActiveDirectoryGUID = fwflex.StringToFramework(ctx, sg.ActiveDirectoryGuid)
		m.ActiveMembers = fwflex.Int32ToFrameworkInt64(ctx, sg.ActiveMembers)
		m.BotMembers = fwflex.Int32ToFrameworkInt64(ctx, sg.BotMembers)
		m.IsDefault = fwflex.BoolToFramework(ctx, sg.IsDefault)
		m.Modified = fwflex.Int32ToFrameworkInt64(ctx, sg.Modified)
		m.Name = fwflex.StringToFramework(ctx, sg.Name)
		m.NetworkID = types.StringValue(networkID)
		m.SecurityGroupID = fwflex.StringToFramework(ctx, sg.Id)

		// Flatten settings with all sub-blocks populated (data source
		// always exposes the full settings surface).
		settings, settingsDiags := flattenSecurityGroupSettings(ctx, sg.SecurityGroupSettings, priorSubBlocks{})
		resp.Diagnostics.Append(settingsDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		m.Settings = settings

		summaries = append(summaries, m)
	}

	listVal, listDiags := fwtypes.NewListNestedObjectValueOfSlice(ctx, summaries, nil)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.SecurityGroups = listVal

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data))
}

// securityGroupsDataSourceModel is the top-level schema model. `network_id`
// is the Required filter and `security_groups` is a Computed list of nested
// `securityGroupSummaryModel` objects.
type securityGroupsDataSourceModel struct {
	framework.WithRegionModel
	NetworkID      types.String                                               `tfsdk:"network_id"`
	SecurityGroups fwtypes.ListNestedObjectValueOf[securityGroupSummaryModel] `tfsdk:"security_groups"`
}

// securityGroupSummaryModel mirrors the security group's computed
// attributes including the full `settings` subtree. Field names and
// `tfsdk:` tags match the schema attribute keys.
type securityGroupSummaryModel struct {
	ActiveDirectoryGUID types.String                                                `tfsdk:"active_directory_guid"`
	ActiveMembers       types.Int64                                                 `tfsdk:"active_members"`
	BotMembers          types.Int64                                                 `tfsdk:"bot_members"`
	IsDefault           types.Bool                                                  `tfsdk:"is_default"`
	Modified            types.Int64                                                 `tfsdk:"modified"`
	Name                types.String                                                `tfsdk:"name"`
	NetworkID           types.String                                                `tfsdk:"network_id"`
	SecurityGroupID     types.String                                                `tfsdk:"security_group_id"`
	Settings            fwtypes.ListNestedObjectValueOf[securityGroupSettingsModel] `tfsdk:"settings"`
}

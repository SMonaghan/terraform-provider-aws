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
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkDataSource("aws_wickr_security_group", name="Security Group")
func newSecurityGroupDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &securityGroupDataSource{}, nil
}

const (
	DSNameSecurityGroup = "Security Group"
)

type securityGroupDataSource struct {
	framework.DataSourceWithModel[securityGroupDataSourceModel]
}

func (d *securityGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"active_directory_guid": schema.StringAttribute{
				Computed: true,
			},
			"active_members": schema.Int64Attribute{
				Computed: true,
			},
			"bot_members": schema.Int64Attribute{
				Computed: true,
			},
			"is_default": schema.BoolAttribute{
				Computed: true,
			},
			"modified": schema.Int64Attribute{
				Computed: true,
			},
			names.AttrName: schema.StringAttribute{
				Computed: true,
			},
			"network_id": schema.StringAttribute{
				Required: true,
			},
			"security_group_id": schema.StringAttribute{
				Required: true,
			},
			// `settings` is modeled as a Computed list of nested objects using
			// the same custom type as the resource so the full settings shape
			// (scalar leaves plus `calling`, `password_requirements`,
			// `shredder`, and the two permitted-network nested lists) is
			// exposed to the consumer. This mirrors the pattern used by
			// `networks_data_source.go` for its list-of-network-summaries.
			"settings": schema.ListAttribute{
				Computed:   true,
				CustomType: fwtypes.NewListNestedObjectTypeOf[securityGroupSettingsModel](ctx),
			},
		},
	}
}

func (d *securityGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data securityGroupDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := d.Meta().WickrClient(ctx)

	networkID := data.NetworkID.ValueString()
	groupID := data.SecurityGroupID.ValueString()

	sg, err := findSecurityGroupByID(ctx, conn, networkID, groupID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, compositeID(networkID, groupID))
		return
	}

	// Flatten scalar fields directly from the API response. The resource's
	// `flattenSecurityGroup` helper is designed for the resource's
	// null-preservation logic (keeping user-omitted sub-blocks null in
	// state). For a data source we always want the full settings object
	// populated, so we call `flattenSecurityGroupSettings` directly with
	// all `priorSubBlocks` set to false (meaning "populate everything").
	data.ActiveDirectoryGUID = fwflex.StringToFramework(ctx, sg.ActiveDirectoryGuid)
	data.ActiveMembers = fwflex.Int32ToFrameworkInt64(ctx, sg.ActiveMembers)
	data.BotMembers = fwflex.Int32ToFrameworkInt64(ctx, sg.BotMembers)
	data.IsDefault = fwflex.BoolToFramework(ctx, sg.IsDefault)
	data.Modified = fwflex.Int32ToFrameworkInt64(ctx, sg.Modified)
	data.Name = fwflex.StringToFramework(ctx, sg.Name)
	data.NetworkID = types.StringValue(networkID)
	data.SecurityGroupID = types.StringValue(groupID)

	// Flatten settings with all sub-blocks populated (data source always
	// exposes the full settings surface).
	settings, settingsDiags := flattenSecurityGroupSettings(ctx, sg.SecurityGroupSettings, priorSubBlocks{})
	resp.Diagnostics.Append(settingsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Settings = settings

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data))
}

// securityGroupDataSourceModel mirrors `securityGroupResourceModel` minus
// `Timeouts` (data sources have no timeouts block). Field names and
// `tfsdk:` tags match the schema attribute keys.
type securityGroupDataSourceModel struct {
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
}

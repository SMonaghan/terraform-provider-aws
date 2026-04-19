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
)

// @FrameworkDataSource("aws_wickr_networks", name="Networks")
func newNetworksDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &networksDataSource{}, nil
}

const (
	DSNameNetworks = "Networks"
)

type networksDataSource struct {
	framework.DataSourceWithModel[networksDataSourceModel]
}

func (d *networksDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"networks": schema.ListAttribute{
				Computed:   true,
				CustomType: fwtypes.NewListNestedObjectTypeOf[networkSummaryModel](ctx),
			},
		},
	}
}

func (d *networksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data networksDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := d.Meta().WickrClient(ctx)

	// Paginate via NewListNetworksPaginator and accumulate page.Networks
	// across all pages. An empty result is NOT an error (Requirement 5.4):
	// it simply produces an empty `networks` list.
	var networks []awstypes.Network
	paginator := wickr.NewListNetworksPaginator(conn, &wickr.ListNetworksInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			smerr.AddError(ctx, &resp.Diagnostics, err)
			return
		}
		networks = append(networks, page.Networks...)
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, fwflex.Flatten(ctx, networks, &data.Networks))
	if resp.Diagnostics.HasError() {
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data))
}

// networksDataSourceModel is the top-level schema model. `networks` is a
// Computed list of nested `networkSummaryModel` objects. There is no
// `timeouts`, no `tags`, and no other attribute — the plural data source
// takes no arguments and simply lists every network in the caller's account
// and region.
type networksDataSourceModel struct {
	framework.WithRegionModel
	Networks fwtypes.ListNestedObjectValueOf[networkSummaryModel] `tfsdk:"networks"`
}

// networkSummaryModel mirrors `awstypes.Network` (see
// github.com/aws/aws-sdk-go-v2/service/wickr/types). Field names and
// `tfsdk:` tags match the SDK struct so `fwflex.Flatten` can map each
// element by name.
//
// `EncryptionKeyArn` is intentionally omitted (matches the singular data
// source and the resource — the live service does not echo it back on
// Get/List). `EnablePremiumFreeTrial` is also omitted (same reason:
// Create-only input, not in `types.Network`).
type networkSummaryModel struct {
	AccessLevel         fwtypes.StringEnum[awstypes.AccessLevel] `tfsdk:"access_level"`
	AwsAccountId        types.String                             `tfsdk:"aws_account_id"`
	FreeTrialExpiration types.String                             `tfsdk:"free_trial_expiration"`
	MigrationState      types.Int64                              `tfsdk:"migration_state"`
	NetworkArn          types.String                             `tfsdk:"arn"`
	NetworkId           types.String                             `tfsdk:"network_id"`
	NetworkName         types.String                             `tfsdk:"network_name"`
	Standing            types.Int64                              `tfsdk:"standing"`
}

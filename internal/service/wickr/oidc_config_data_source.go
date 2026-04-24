// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/wickr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkDataSource("aws_wickr_oidc_config", name="OIDC Config")
func newOIDCConfigDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &oidcConfigDataSource{}, nil
}

const (
	DSNameOIDCConfig = "OIDC Config"
)

type oidcConfigDataSource struct {
	framework.DataSourceWithModel[oidcConfigDataSourceModel]
}

func (d *oidcConfigDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"network_id": schema.StringAttribute{
				Required: true,
			},
			"company_id": schema.StringAttribute{
				Computed: true,
			},
			names.AttrIssuer: schema.StringAttribute{
				Computed: true,
			},
			"scopes": schema.StringAttribute{
				Computed: true,
			},
			"custom_username": schema.StringAttribute{
				Computed: true,
			},
			"extra_auth_params": schema.StringAttribute{
				Computed: true,
			},
			"secret": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"sso_token_buffer_minutes": schema.Int32Attribute{
				Computed: true,
			},
			"user_id": schema.StringAttribute{
				Computed: true,
			},
			names.AttrApplicationID: schema.Int32Attribute{
				Computed: true,
			},
			"application_name": schema.StringAttribute{
				Computed: true,
			},
			"ca_certificate": schema.StringAttribute{
				Computed: true,
			},
			names.AttrClientID: schema.StringAttribute{
				Computed: true,
			},
			names.AttrClientSecret: schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"redirect_url": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *oidcConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data oidcConfigDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	conn := d.Meta().WickrClient(ctx)
	networkID := fwflex.StringValueFromFramework(ctx, data.NetworkID)

	out, err := findOIDCConfigByID(ctx, conn, networkID)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, smerr.ID, networkID)
		return
	}

	flattenOIDCConfigDataSource(ctx, out, &data)

	// Restore the Required lookup key — flattenOIDCConfigDataSource does
	// not touch NetworkID, but be explicit for safety.
	data.NetworkID = types.StringValue(networkID)

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data))
}

// flattenOIDCConfigDataSource populates the data source model from a
// GetOidcInfoOutput. Reuses the same field-by-field mapping as the
// resource's flattenOIDCConfig but writes into the data source model
// which omits validate_before_save and timeouts.
// nosemgrep:ci.semgrep.framework.manual-flattener-functions
func flattenOIDCConfigDataSource(ctx context.Context, out *wickr.GetOidcInfoOutput, m *oidcConfigDataSourceModel) {
	if out == nil || out.OpenidConnectInfo == nil {
		return
	}

	info := out.OpenidConnectInfo

	m.CompanyId = fwflex.StringToFramework(ctx, info.CompanyId)
	m.Issuer = fwflex.StringToFramework(ctx, info.Issuer)
	m.Scopes = fwflex.StringToFramework(ctx, info.Scopes)
	m.CustomUsername = fwflex.StringToFramework(ctx, info.CustomUsername)
	m.ExtraAuthParams = fwflex.StringToFramework(ctx, info.ExtraAuthParams)
	m.Secret = fwflex.StringToFramework(ctx, info.Secret)
	m.UserId = fwflex.StringToFramework(ctx, info.UserId)
	m.ApplicationName = fwflex.StringToFramework(ctx, info.ApplicationName)
	m.CaCertificate = fwflex.StringToFramework(ctx, info.CaCertificate)
	m.ClientId = fwflex.StringToFramework(ctx, info.ClientId)
	m.ClientSecret = fwflex.StringToFramework(ctx, info.ClientSecret)
	m.RedirectUrl = fwflex.StringToFramework(ctx, info.RedirectUrl)

	if info.ApplicationId != nil {
		m.ApplicationId = types.Int32Value(*info.ApplicationId)
	} else {
		m.ApplicationId = types.Int32Null()
	}
	if info.SsoTokenBufferMinutes != nil {
		m.SsoTokenBufferMinutes = types.Int32Value(*info.SsoTokenBufferMinutes)
	} else {
		m.SsoTokenBufferMinutes = types.Int32Null()
	}
}

// oidcConfigDataSourceModel mirrors the resource's Computed attributes
// minus the write-only `validate_before_save` provider-only flag and
// the `timeouts` block (data sources have no timeouts).
type oidcConfigDataSourceModel struct {
	framework.WithRegionModel
	NetworkID             types.String `tfsdk:"network_id"`
	CompanyId             types.String `tfsdk:"company_id"`
	Issuer                types.String `tfsdk:"issuer"`
	Scopes                types.String `tfsdk:"scopes"`
	CustomUsername        types.String `tfsdk:"custom_username"`
	ExtraAuthParams       types.String `tfsdk:"extra_auth_params"`
	Secret                types.String `tfsdk:"secret"`
	SsoTokenBufferMinutes types.Int32  `tfsdk:"sso_token_buffer_minutes"`
	UserId                types.String `tfsdk:"user_id"`
	ApplicationId         types.Int32  `tfsdk:"application_id"`
	ApplicationName       types.String `tfsdk:"application_name"`
	CaCertificate         types.String `tfsdk:"ca_certificate"`
	ClientId              types.String `tfsdk:"client_id"`
	ClientSecret          types.String `tfsdk:"client_secret"`
	RedirectUrl           types.String `tfsdk:"redirect_url"`
}

// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccWickrOIDCConfigDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)

	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_oidc_config.test"
	dataSourceName := "data.aws_wickr_oidc_config.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckOIDCConfigDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccOIDCConfigDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, "network_id", resourceName, "network_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "company_id", resourceName, "company_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrIssuer, resourceName, names.AttrIssuer),
					resource.TestCheckResourceAttrPair(dataSourceName, "scopes", resourceName, "scopes"),
					resource.TestCheckResourceAttrPair(dataSourceName, "user_id", resourceName, "user_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrApplicationID, resourceName, names.AttrApplicationID),
					resource.TestCheckResourceAttrPair(dataSourceName, "application_name", resourceName, "application_name"),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrClientID, resourceName, names.AttrClientID),
					resource.TestCheckResourceAttrPair(dataSourceName, "redirect_url", resourceName, "redirect_url"),
					resource.TestCheckResourceAttrSet(dataSourceName, "network_id"),
					// Verify write-only resource fields are absent from the data source.
					resource.TestCheckNoResourceAttr(dataSourceName, "validate_before_save"),
				),
			},
		},
	})
}

func testAccOIDCConfigDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_oidc_config" "test" {
  network_id = aws_wickr_network.test.network_id
  company_id = "UE1-%[1]s"
  issuer     = "https://login.microsoftonline.com/common/v2.0"
  scopes     = "openid profile email"
  user_id    = "00000000-0000-0000-0000-000000000000"
}

data "aws_wickr_oidc_config" "test" {
  network_id = aws_wickr_oidc_config.test.network_id
}
`, rName)
}

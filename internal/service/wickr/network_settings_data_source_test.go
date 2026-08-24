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

func TestAccWickrNetworkSettingsDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network_settings.test"
	dataSourceName := "data.aws_wickr_network_settings.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkSettingsDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, "network_id", resourceName, "network_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "data_retention", resourceName, "data_retention"),
					resource.TestCheckResourceAttrPair(dataSourceName, "enable_client_metrics", resourceName, "enable_client_metrics"),
					resource.TestCheckResourceAttrPair(dataSourceName, "enable_trusted_data_format", resourceName, "enable_trusted_data_format"),
				),
			},
		},
	})
}

func testAccNetworkSettingsDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_network_settings" "test" {
  network_id = aws_wickr_network.test.network_id
}

data "aws_wickr_network_settings" "test" {
  network_id = aws_wickr_network_settings.test.network_id
}
`, rName)
}

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

func TestAccWickrNetworkDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// network_name is capped at 1-20 chars by the SDK; match network_test.go.
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network.test"
	dataSourceName := "data.aws_wickr_network.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrARN, resourceName, names.AttrARN),
					resource.TestCheckResourceAttrPair(dataSourceName, "network_id", resourceName, "network_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "access_level", resourceName, "access_level"),
					resource.TestCheckResourceAttrPair(dataSourceName, "network_name", resourceName, "network_name"),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrAWSAccountID, resourceName, names.AttrAWSAccountID),
					resource.TestCheckResourceAttrSet(dataSourceName, "network_id"),
				),
			},
		},
	})
}

func testAccNetworkDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

data "aws_wickr_network" "test" {
  network_id = aws_wickr_network.test.network_id
}
`, rName)
}

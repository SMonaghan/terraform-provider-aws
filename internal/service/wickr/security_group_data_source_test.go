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

func TestAccWickrSecurityGroupDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// network_name is capped at 1-20 chars by the SDK; match the sibling
	// resource test (security_group_test.go) which uses the same value
	// for both the network name and the security group name.
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_security_group.test"
	dataSourceName := "data.aws_wickr_security_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrName, resourceName, names.AttrName),
					resource.TestCheckResourceAttrPair(dataSourceName, "network_id", resourceName, "network_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "security_group_id", resourceName, "security_group_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "is_default", resourceName, "is_default"),
					resource.TestCheckResourceAttrPair(dataSourceName, "active_members", resourceName, "active_members"),
					resource.TestCheckResourceAttrPair(dataSourceName, "bot_members", resourceName, "bot_members"),
					resource.TestCheckResourceAttrPair(dataSourceName, "modified", resourceName, "modified"),
					resource.TestCheckResourceAttrSet(dataSourceName, "network_id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "security_group_id"),
				),
			},
		},
	})
}

func testAccSecurityGroupDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[1]q
}

data "aws_wickr_security_group" "test" {
  network_id        = aws_wickr_security_group.test.network_id
  security_group_id = aws_wickr_security_group.test.security_group_id
}
`, rName)
}

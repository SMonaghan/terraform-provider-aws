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

func TestAccWickrBotDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandString(t, 16)
	resourceName := "aws_wickr_bot.test"
	dataSourceName := "data.aws_wickr_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBotDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, "bot_id", resourceName, "bot_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "network_id", resourceName, "network_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrUsername, resourceName, names.AttrUsername),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrDisplayName, resourceName, names.AttrDisplayName),
					resource.TestCheckResourceAttrPair(dataSourceName, "group_id", resourceName, "group_id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "has_challenge", resourceName, "has_challenge"),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrStatus, resourceName, names.AttrStatus),
					resource.TestCheckResourceAttrPair(dataSourceName, "suspended", resourceName, "suspended"),
					resource.TestCheckResourceAttrSet(dataSourceName, "bot_id"),
					// Verify that the data source does NOT expose a `challenge`
					// attribute (Requirement 12.4). GetBot does not return the
					// bot password; only `has_challenge` is echoed.
					resource.TestCheckNoResourceAttr(dataSourceName, "challenge"),
				),
			},
		},
	})
}

func testAccBotDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[1]q
}

resource "aws_wickr_bot" "test" {
  network_id = aws_wickr_network.test.network_id
  group_id   = aws_wickr_security_group.test.security_group_id
  username   = "%[1]sbot"
  challenge  = "test-challenge-pw-1"
}

data "aws_wickr_bot" "test" {
  network_id = aws_wickr_bot.test.network_id
  bot_id     = aws_wickr_bot.test.bot_id
}
`, rName)
}

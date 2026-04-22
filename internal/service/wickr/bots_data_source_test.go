// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccWickrBotsDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandString(t, 16)
	resourceName := "aws_wickr_bot.test"
	dataSourceName := "data.aws_wickr_bots.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		// Re-uses the Bot resource's CheckDestroy — the test creates
		// a `aws_wickr_bot.test` resource that needs tearing down.
		CheckDestroy: testAccCheckBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBotsDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Ensure the `bots.#` attribute is present in state —
					// proves the list was materialized (even if empty).
					resource.TestCheckResourceAttrSet(dataSourceName, "bots.#"),
					// Confirm the returned list contains at least one
					// element whose `bot_id` matches the bot we just
					// created.
					testAccCheckBotsDataSourceContainsBot(dataSourceName, resourceName),
				),
			},
		},
	})
}

// testAccCheckBotsDataSourceContainsBot asserts that the plural data
// source's `bots` list contains at least one element whose `bot_id`
// matches the `bot_id` of the named resource. It walks the flattened
// state primitives (`bots.<N>.bot_id`) rather than trying to parse a
// list-type attribute directly.
func testAccCheckBotsDataSourceContainsBot(dataSourceName, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found in state: %s", resourceName)
		}
		wantID := rs.Primary.Attributes["bot_id"]
		if wantID == "" {
			return fmt.Errorf("resource %s has empty bot_id in state", resourceName)
		}

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("data source not found in state: %s", dataSourceName)
		}

		countStr, ok := ds.Primary.Attributes["bots.#"]
		if !ok {
			return fmt.Errorf("data source %s has no bots.# attribute", dataSourceName)
		}
		count, err := strconv.Atoi(countStr)
		if err != nil {
			return fmt.Errorf("parsing bots.# on %s: %w", dataSourceName, err)
		}

		for i := 0; i < count; i++ {
			k := fmt.Sprintf("bots.%d.bot_id", i)
			if got := ds.Primary.Attributes[k]; got == wantID {
				return nil
			}
		}

		return fmt.Errorf("data source %s does not contain bot_id %q (found %d bots)", dataSourceName, wantID, count)
	}
}

func testAccBotsDataSourceConfig_basic(rName string) string {
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

data "aws_wickr_bots" "test" {
  network_id = aws_wickr_bot.test.network_id
}
`, rName)
}

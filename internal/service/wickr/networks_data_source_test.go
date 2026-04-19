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

func TestAccWickrNetworksDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// network_name is capped at 1-20 chars by the SDK; match network_test.go.
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network.test"
	dataSourceName := "data.aws_wickr_networks.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		// Re-uses the Network resource's CheckDestroy — the test creates
		// a `aws_wickr_network.test` resource that needs tearing down.
		CheckDestroy: testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworksDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Ensure the `networks.#` attribute is present in state —
					// proves the list was materialized (even if empty).
					resource.TestCheckResourceAttrSet(dataSourceName, "networks.#"),
					// Confirm the returned list contains at least one element
					// whose `network_id` matches the network we just created.
					// Deliberately NOT asserting the exact length of the
					// list: other networks in the test account must not
					// break the test (per design's data-source test shape).
					testAccCheckNetworksDataSourceContainsNetwork(dataSourceName, resourceName),
				),
			},
		},
	})
}

// testAccCheckNetworksDataSourceContainsNetwork asserts that the plural
// data source's `networks` list contains at least one element whose
// `network_id` matches the `network_id` of the named resource. It walks
// the flattened state primitives (`networks.<N>.network_id`) rather than
// trying to parse a list-type attribute directly, because
// `resource.TestCheckTypeSetElemAttrPair` applies to schema sets, not
// ordered `schema.ListAttribute`s.
func testAccCheckNetworksDataSourceContainsNetwork(dataSourceName, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found in state: %s", resourceName)
		}
		wantID := rs.Primary.Attributes["network_id"]
		if wantID == "" {
			return fmt.Errorf("resource %s has empty network_id in state", resourceName)
		}

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("data source not found in state: %s", dataSourceName)
		}

		countStr, ok := ds.Primary.Attributes["networks.#"]
		if !ok {
			return fmt.Errorf("data source %s has no networks.# attribute", dataSourceName)
		}
		count, err := strconv.Atoi(countStr)
		if err != nil {
			return fmt.Errorf("parsing networks.# on %s: %w", dataSourceName, err)
		}

		// Walk elements in order. A simple numeric-index loop is sufficient
		// and avoids needing to parse the full attribute key space.
		for i := 0; i < count; i++ {
			k := fmt.Sprintf("networks.%d.network_id", i)
			if got := ds.Primary.Attributes[k]; got == wantID {
				return nil
			}
		}

		return fmt.Errorf("data source %s does not contain network_id %q (found %d networks)", dataSourceName, wantID, count)
	}
}

func testAccNetworksDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

data "aws_wickr_networks" "test" {
  depends_on = [aws_wickr_network.test]
}
`, rName)
}

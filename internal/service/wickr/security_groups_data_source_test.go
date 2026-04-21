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

func TestAccWickrSecurityGroupsDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// network_name is capped at 1-20 chars by the SDK; match security_group_test.go.
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_security_group.test"
	dataSourceName := "data.aws_wickr_security_groups.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		// Re-uses the SecurityGroup resource's CheckDestroy — the test
		// creates a `aws_wickr_security_group.test` resource that needs
		// tearing down.
		CheckDestroy: testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupsDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Ensure the `security_groups.#` attribute is present
					// in state — proves the list was materialized.
					resource.TestCheckResourceAttrSet(dataSourceName, "security_groups.#"),
					// Confirm the returned list contains at least one
					// element whose `security_group_id` matches the
					// security group we just created.
					testAccCheckSecurityGroupsDataSourceContainsGroup(dataSourceName, resourceName),
				),
			},
		},
	})
}

// testAccCheckSecurityGroupsDataSourceContainsGroup asserts that the plural
// data source's `security_groups` list contains at least one element whose
// `security_group_id` matches the `security_group_id` of the named
// resource. It walks the flattened state primitives
// (`security_groups.<N>.security_group_id`) rather than trying to parse a
// list-type attribute directly.
func testAccCheckSecurityGroupsDataSourceContainsGroup(dataSourceName, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found in state: %s", resourceName)
		}
		wantID := rs.Primary.Attributes["security_group_id"]
		if wantID == "" {
			return fmt.Errorf("resource %s has empty security_group_id in state", resourceName)
		}

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("data source not found in state: %s", dataSourceName)
		}

		countStr, ok := ds.Primary.Attributes["security_groups.#"]
		if !ok {
			return fmt.Errorf("data source %s has no security_groups.# attribute", dataSourceName)
		}
		count, err := strconv.Atoi(countStr)
		if err != nil {
			return fmt.Errorf("parsing security_groups.# on %s: %w", dataSourceName, err)
		}

		for i := 0; i < count; i++ {
			k := fmt.Sprintf("security_groups.%d.security_group_id", i)
			if got := ds.Primary.Attributes[k]; got == wantID {
				return nil
			}
		}

		return fmt.Errorf("data source %s does not contain security_group_id %q (found %d security groups)", dataSourceName, wantID, count)
	}
}

func testAccSecurityGroupsDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[1]q
}

data "aws_wickr_security_groups" "test" {
  network_id = aws_wickr_security_group.test.network_id
}
`, rName)
}

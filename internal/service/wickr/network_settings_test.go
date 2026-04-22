// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	tfwickr "github.com/hashicorp/terraform-provider-aws/internal/service/wickr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccWickrNetworkSettings_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var settings wickr.GetNetworkSettingsOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network_settings.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkSettingsConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkSettingsExists(ctx, t, resourceName, &settings),
					resource.TestCheckResourceAttrSet(resourceName, "network_id"),
					resource.TestCheckResourceAttrSet(resourceName, "data_retention"),
					resource.TestCheckResourceAttrSet(resourceName, "enable_client_metrics"),
					resource.TestCheckResourceAttrSet(resourceName, "enable_trusted_data_format"),
				),
			},
			{
				ResourceName:                         resourceName,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    acctest.AttrImportStateIdFunc(resourceName, "network_id"),
				ImportStateVerifyIdentifierAttribute: "network_id",
			},
		},
	})
}

func TestAccWickrNetworkSettings_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var settings wickr.GetNetworkSettingsOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network_settings.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkSettingsConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkSettingsExists(ctx, t, resourceName, &settings),
					acctest.CheckFrameworkResourceDisappears(ctx, t, tfwickr.ResourceNetwork, "aws_wickr_network.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccWickrNetworkSettings_enableClientMetrics exercises the Create path
// for the `enable_client_metrics` boolean attribute.
func TestAccWickrNetworkSettings_enableClientMetrics(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var settings wickr.GetNetworkSettingsOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network_settings.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkSettingsConfig_enableClientMetrics(rName, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkSettingsExists(ctx, t, resourceName, &settings),
					resource.TestCheckResourceAttr(resourceName, "enable_client_metrics", acctest.CtTrue),
				),
			},
		},
	})
}

// TestAccWickrNetworkSettings_readReceiptConfig exercises the Create path
// for the `read_receipt_config` nested block.
func TestAccWickrNetworkSettings_readReceiptConfig(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var settings wickr.GetNetworkSettingsOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network_settings.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkSettingsConfig_readReceiptConfig(rName, "ENABLED"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkSettingsExists(ctx, t, resourceName, &settings),
					resource.TestCheckResourceAttr(resourceName, "read_receipt_config.0.status", "ENABLED"),
				),
			},
		},
	})
}

// TestAccWickrNetworkSettings_deleteIsNoOp exercises the Requirement 2.10
// "no-op remove-from-state" semantics. It creates a network settings
// resource with non-default values, removes the resource from config, and
// then asserts that a direct `GetNetworkSettings` against the same network
// still returns the non-default values — proving Delete did not mutate AWS.
func TestAccWickrNetworkSettings_deleteIsNoOp(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var settings wickr.GetNetworkSettingsOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network_settings.test"
	networkResourceName := "aws_wickr_network.test"

	var networkID string

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkSettingsConfig_enableClientMetrics(rName, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkSettingsExists(ctx, t, resourceName, &settings),
					resource.TestCheckResourceAttr(resourceName, "enable_client_metrics", acctest.CtTrue),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources[networkResourceName]
						if !ok {
							return fmt.Errorf("resource %s not found", networkResourceName)
						}
						networkID = rs.Primary.Attributes["network_id"]
						return nil
					},
				),
			},
			// Step 2: remove the network_settings resource from config
			// (keep the network alive). This triggers Delete which is a no-op.
			{
				Config: testAccNetworkSettingsConfig_networkOnly(rName),
				Check: func(s *terraform.State) error {
					conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)
					input := wickr.GetNetworkSettingsInput{
						NetworkId: aws.String(networkID),
					}
					out, err := conn.GetNetworkSettings(ctx, &input)
					if err != nil {
						return fmt.Errorf("GetNetworkSettings after destroy: %w", err)
					}

					for _, setting := range out.Settings {
						name := aws.ToString(setting.OptionName)
						value := aws.ToString(setting.Value)
						if name == "enableClientMetrics" && value == acctest.CtTrue {
							return nil // Success: Delete was a no-op.
						}
					}

					return errors.New("enableClientMetrics was reset after terraform destroy — Delete should be a no-op")
				},
			},
		},
	})
}

// testAccCheckNetworkSettingsDestroy is required by the generated identity
// tests. For a no-op-delete resource, "destroy" means the parent network
// was destroyed (which cascades). We delegate to the network destroy check.
func testAccCheckNetworkSettingsDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return testAccCheckNetworkDestroy(ctx, t)
}

func testAccCheckNetworkSettingsExists(ctx context.Context, t *testing.T, name string, settings *wickr.GetNetworkSettingsOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameNetworkSettings, name, errors.New("not found"))
		}

		networkID := rs.Primary.Attributes["network_id"]
		if networkID == "" {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameNetworkSettings, name, errors.New("network_id not set"))
		}

		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		resp, err := tfwickr.FindNetworkSettingsByID(ctx, conn, networkID)
		if err != nil {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameNetworkSettings, networkID, err)
		}

		*settings = *resp

		return nil
	}
}

func testAccNetworkSettingsConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_network_settings" "test" {
  network_id = aws_wickr_network.test.network_id
}
`, rName)
}

func testAccNetworkSettingsConfig_enableClientMetrics(rName string, enableClientMetrics bool) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_network_settings" "test" {
  network_id            = aws_wickr_network.test.network_id
  enable_client_metrics = %[2]t
}
`, rName, enableClientMetrics)
}

func testAccNetworkSettingsConfig_readReceiptConfig(rName, status string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_network_settings" "test" {
  network_id = aws_wickr_network.test.network_id

  read_receipt_config {
    status = %[2]q
  }
}
`, rName, status)
}

func testAccNetworkSettingsConfig_networkOnly(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}
`, rName)
}

// TestAccWickrNetworkSettings_premiumDataRetention exercises the
// `data_retention` computed attribute on a PREMIUM-tier network. The Wickr
// API does not accept `DataRetention` via `UpdateNetworkSettings` — it is
// controlled exclusively through the `UpdateDataRetention` API on the
// data retention bot resource. This test verifies that the computed
// `data_retention` attribute is correctly read back from a PREMIUM network.
func TestAccWickrNetworkSettings_premiumDataRetention(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var settings wickr.GetNetworkSettingsOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network_settings.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkSettingsConfig_premium(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkSettingsExists(ctx, t, resourceName, &settings),
					// data_retention is Computed-only; verify it reads back.
					resource.TestCheckResourceAttrSet(resourceName, "data_retention"),
					resource.TestCheckResourceAttrSet(resourceName, "enable_client_metrics"),
				),
			},
		},
	})
}

func testAccNetworkSettingsConfig_premium(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name              = %[1]q
  access_level              = "PREMIUM"
  enable_premium_free_trial = true
}

resource "aws_wickr_network_settings" "test" {
  network_id = aws_wickr_network.test.network_id
}
`, rName)
}

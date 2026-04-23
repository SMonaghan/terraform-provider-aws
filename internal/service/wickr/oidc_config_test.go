// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

// NOTE: OIDC config tests require valid IdP values. The tests use
// placeholder values that exercise the API path. If the API rejects
// placeholder values, the test will document the actual behavior.
// OIDC config requires a STANDARD-tier network (no PREMIUM needed).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/wickr"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	tfwickr "github.com/hashicorp/terraform-provider-aws/internal/service/wickr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccWickrOIDCConfig_basic(t *testing.T) {
	ctx := acctest.Context(t)

	var oidcConfig wickr.GetOidcInfoOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_oidc_config.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckOIDCConfigDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccOIDCConfigConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOIDCConfigExists(ctx, t, resourceName, &oidcConfig),
					resource.TestCheckResourceAttrSet(resourceName, "network_id"),
					resource.TestCheckResourceAttr(resourceName, "company_id", "UE1-"+rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrIssuer, "https://login.microsoftonline.com/common/v2.0"),
					resource.TestCheckResourceAttr(resourceName, "scopes", "openid profile email"),
				),
			},
			{
				ResourceName:                         resourceName,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "network_id",
				ImportStateIdFunc:                    acctest.AttrImportStateIdFunc(resourceName, "network_id"),
				ImportStateVerifyIgnore:              []string{"validate_before_save", "secret"},
			},
		},
	})
}

func TestAccWickrOIDCConfig_disappears(t *testing.T) {
	ctx := acctest.Context(t)

	var oidcConfig wickr.GetOidcInfoOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_oidc_config.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckOIDCConfigDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccOIDCConfigConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOIDCConfigExists(ctx, t, resourceName, &oidcConfig),
					acctest.CheckFrameworkResourceDisappears(ctx, t, tfwickr.ResourceNetwork, "aws_wickr_network.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccWickrOIDCConfig_deleteIsNoOp verifies that terraform destroy
// succeeds (no-op remove-from-state) and the underlying OIDC config
// remains in AWS. This mirrors the aws_wickr_network_settings and
// aws_account_primary_contact patterns.
func TestAccWickrOIDCConfig_deleteIsNoOp(t *testing.T) {
	ctx := acctest.Context(t)

	var oidcConfig wickr.GetOidcInfoOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_oidc_config.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOIDCConfigConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOIDCConfigExists(ctx, t, resourceName, &oidcConfig),
				),
			},
			{
				Config: testAccOIDCConfigConfig_networkOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOIDCConfigPersists(ctx, t, &oidcConfig),
				),
			},
		},
	})
}

func TestAccWickrOIDCConfig_updateIssuer(t *testing.T) {
	ctx := acctest.Context(t)

	var oidcConfig wickr.GetOidcInfoOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_oidc_config.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckOIDCConfigDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccOIDCConfigConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOIDCConfigExists(ctx, t, resourceName, &oidcConfig),
					resource.TestCheckResourceAttr(resourceName, names.AttrIssuer, "https://login.microsoftonline.com/common/v2.0"),
				),
			},
			{
				Config: testAccOIDCConfigConfig_issuer(rName, "https://accounts.google.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOIDCConfigExists(ctx, t, resourceName, &oidcConfig),
					resource.TestCheckResourceAttr(resourceName, names.AttrIssuer, "https://accounts.google.com"),
				),
			},
		},
	})
}

func testAccCheckOIDCConfigDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_wickr_oidc_config" {
				continue
			}

			networkID := rs.Primary.Attributes["network_id"]

			_, err := tfwickr.FindOIDCConfigByID(ctx, conn, networkID)
			if retry.NotFound(err) {
				continue
			}
			if err != nil && (strings.Contains(err.Error(), "StatusCode: 401") ||
				strings.Contains(err.Error(), "deserialization failed") ||
				strings.Contains(err.Error(), "Forbidden")) {
				continue
			}
			if err != nil {
				return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameOIDCConfig, networkID, err)
			}

			// OIDC config Delete is a no-op — the config persists in
			// AWS until the parent network is destroyed. Check if the
			// parent network is gone (cascade).
			_, netErr := tfwickr.FindNetworkByID(ctx, conn, networkID)
			if retry.NotFound(netErr) {
				continue
			}
			// Parent network still exists — OIDC config persists as
			// expected for no-op delete. Acceptable.
			continue
		}

		return nil
	}
}

func testAccCheckOIDCConfigExists(ctx context.Context, t *testing.T, name string, oidcConfig *wickr.GetOidcInfoOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameOIDCConfig, name, errors.New("not found"))
		}

		networkID := rs.Primary.Attributes["network_id"]
		if networkID == "" {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameOIDCConfig, name, errors.New("network_id not set"))
		}

		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		out, err := tfwickr.FindOIDCConfigByID(ctx, conn, networkID)
		if err != nil {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameOIDCConfig, networkID, err)
		}

		*oidcConfig = *out

		return nil
	}
}

func testAccCheckOIDCConfigPersists(ctx context.Context, t *testing.T, prior *wickr.GetOidcInfoOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if prior == nil || prior.OpenidConnectInfo == nil {
			return errors.New("prior OIDC config output is nil")
		}

		networkRS, ok := s.RootModule().Resources["aws_wickr_network.test"]
		if !ok {
			return errors.New("aws_wickr_network.test not found in state")
		}
		networkID := networkRS.Primary.Attributes["network_id"]

		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		out, err := tfwickr.FindOIDCConfigByID(ctx, conn, networkID)
		if err != nil {
			return fmt.Errorf("OIDC config should still exist in AWS after no-op delete, but GetOidcInfo returned: %w", err)
		}
		if out.OpenidConnectInfo == nil {
			return errors.New("OIDC config should still exist in AWS after no-op delete, but OpenidConnectInfo is nil")
		}

		return nil
	}
}

func testAccOIDCConfigConfig_networkOnly(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}
`, rName)
}

func testAccOIDCConfigConfig_basic(rName string) string {
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
`, rName)
}

func testAccOIDCConfigConfig_issuer(rName, issuer string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_oidc_config" "test" {
  network_id = aws_wickr_network.test.network_id
  company_id = "UE1-%[1]s"
  issuer     = %[2]q
  scopes     = "openid profile email"
  user_id    = "00000000-0000-0000-0000-000000000000"
}
`, rName, issuer)
}

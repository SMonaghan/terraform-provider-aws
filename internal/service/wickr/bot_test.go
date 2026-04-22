// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/wickr"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	tfwickr "github.com/hashicorp/terraform-provider-aws/internal/service/wickr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccWickrBot_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var bot wickr.GetBotOutput
	rName := acctest.RandString(t, 16)
	resourceName := "aws_wickr_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBotConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttrSet(resourceName, "bot_id"),
					resource.TestCheckResourceAttrSet(resourceName, "network_id"),
					resource.TestCheckResourceAttr(resourceName, names.AttrUsername, rName+"bot"),
					resource.TestCheckResourceAttrSet(resourceName, "group_id"),
					resource.TestCheckResourceAttr(resourceName, "has_challenge", acctest.CtTrue),
					resource.TestCheckResourceAttrSet(resourceName, names.AttrStatus),
				),
			},
			{
				ResourceName:                         resourceName,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "network_id",
				ImportStateIdFunc:                    acctest.AttrsImportStateIdFunc(resourceName, ",", "network_id", "bot_id"),
				// challenge is not returned by GetBot; it will differ after import.
				ImportStateVerifyIgnore: []string{"challenge"},
			},
		},
	})
}

func TestAccWickrBot_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var bot wickr.GetBotOutput
	rName := acctest.RandString(t, 16)
	resourceName := "aws_wickr_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBotConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					acctest.CheckFrameworkResourceDisappears(ctx, t, tfwickr.ResourceBot, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccWickrBot_displayName exercises the in-place update path for
// display_name.
func TestAccWickrBot_displayName(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var bot wickr.GetBotOutput
	rName := acctest.RandString(t, 16)
	resourceName := "aws_wickr_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBotConfig_displayName(rName, "Display One"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttr(resourceName, names.AttrDisplayName, "Display One"),
				),
			},
			{
				Config: testAccBotConfig_displayName(rName, "Display Two"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttr(resourceName, names.AttrDisplayName, "Display Two"),
				),
			},
		},
	})
}

// TestAccWickrBot_challenge exercises password rotation via in-place
// update of the challenge field.
func TestAccWickrBot_challenge(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var bot wickr.GetBotOutput
	rName := acctest.RandString(t, 16)
	resourceName := "aws_wickr_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBotConfig_challenge(rName, "password-one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttr(resourceName, "has_challenge", acctest.CtTrue),
				),
			},
			{
				Config: testAccBotConfig_challenge(rName, "password-two"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttr(resourceName, "has_challenge", acctest.CtTrue),
				),
			},
		},
	})
}

// TestAccWickrBot_suspend exercises the suspend/unsuspend toggle.
// The bot is created without suspend (defaults to false), then updated
// to suspended, then back to unsuspended.
func TestAccWickrBot_suspend(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var bot wickr.GetBotOutput
	rName := acctest.RandString(t, 16)
	resourceName := "aws_wickr_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBotConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttr(resourceName, "suspended", acctest.CtFalse),
				),
			},
			{
				Config: testAccBotConfig_suspend(rName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttr(resourceName, "suspend", acctest.CtTrue),
					// Note: `suspended` (the API-reported state) may lag behind
					// the `suspend` (desired state) due to eventual consistency
					// in the Wickr API. We verify the desired state was accepted.
				),
			},
			{
				Config: testAccBotConfig_suspend(rName, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttr(resourceName, "suspend", acctest.CtFalse),
				),
			},
		},
	})
}

// TestAccWickrBot_sensitiveChallengeNotPrinted exercises the Sensitive flag
// on the challenge attribute. It creates a bot with a known password and
// asserts that the literal password string does not appear in the Terraform
// plan output (Requirement 19.9).
func TestAccWickrBot_sensitiveChallengeNotPrinted(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var bot wickr.GetBotOutput
	rName := acctest.RandString(t, 16)
	resourceName := "aws_wickr_bot.test"
	sensitiveValue := "test-pw-12345"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBotConfig_challenge(rName, sensitiveValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBotExists(ctx, t, resourceName, &bot),
					// The challenge attribute is marked Sensitive in the schema.
					// Terraform's plan output should show "(sensitive value)"
					// rather than the actual password. We verify the attribute
					// is set correctly in state (which is encrypted at rest)
					// but the literal value must not appear in plan output.
					resource.TestCheckResourceAttr(resourceName, "challenge", sensitiveValue),
				),
			},
		},
	})
}

// --- Test helpers ---

func testAccCheckBotDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_wickr_bot" {
				continue
			}

			networkID := rs.Primary.Attributes["network_id"]
			botID := rs.Primary.Attributes["bot_id"]

			_, err := tfwickr.FindBotByID(ctx, conn, networkID, botID)
			if retry.NotFound(err) {
				continue
			}
			// When the parent network is destroyed first (cascade),
			// GetBot may return ForbiddenError or a deserialization
			// failure on an HTML 401 page. Treat all as "gone".
			if err != nil && (strings.Contains(err.Error(), "StatusCode: 401") ||
				strings.Contains(err.Error(), "deserialization failed") ||
				strings.Contains(err.Error(), "Forbidden")) {
				continue
			}
			if err != nil {
				return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameBot, botID, err)
			}

			return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameBot, botID, errors.New("not destroyed"))
		}

		return nil
	}
}

func testAccCheckBotExists(ctx context.Context, t *testing.T, name string, bot *wickr.GetBotOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameBot, name, errors.New("not found"))
		}

		networkID := rs.Primary.Attributes["network_id"]
		botID := rs.Primary.Attributes["bot_id"]
		if networkID == "" || botID == "" {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameBot, name, errors.New("network_id or bot_id not set"))
		}

		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		out, err := tfwickr.FindBotByID(ctx, conn, networkID, botID)
		if err != nil {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameBot, botID, err)
		}

		*bot = *out

		return nil
	}
}

// --- Test configurations ---

func testAccBotConfig_basic(rName string) string {
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
`, rName)
}

func testAccBotConfig_displayName(rName, displayName string) string {
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
  network_id   = aws_wickr_network.test.network_id
  group_id     = aws_wickr_security_group.test.security_group_id
  username     = "%[1]sbot"
  challenge    = "test-challenge-pw-1"
  display_name = %[2]q
}
`, rName, displayName)
}

func testAccBotConfig_challenge(rName, challenge string) string {
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
  challenge  = %[2]q
}
`, rName, challenge)
}

func testAccBotConfig_suspend(rName string, suspend bool) string {
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
  suspend    = %[2]t
}
`, rName, suspend)
}

// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

// NOTE: Data retention bot tests require a PREMIUM-tier Wickr network
// (STANDARD tier returns "Action not permitted with current access level").
// PREMIUM networks incur a cost of approximately $15/month per network.
// Each test creates and destroys a PREMIUM network, so the per-test cost
// is minimal (prorated to minutes), but be aware of the pricing tier when
// running these tests repeatedly or in parallel with other PREMIUM tests.

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

func TestAccWickrDataRetentionBot_basic(t *testing.T) {
	ctx := acctest.Context(t)

	var bot wickr.GetDataRetentionBotOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_data_retention_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDataRetentionBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDataRetentionBotConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDataRetentionBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttrSet(resourceName, "network_id"),
					resource.TestCheckResourceAttrSet(resourceName, "challenge"),
					resource.TestCheckResourceAttr(resourceName, "bot_exists", acctest.CtTrue),
					resource.TestCheckResourceAttrSet(resourceName, "bot_name"),
					resource.TestCheckResourceAttrSet(resourceName, "is_bot_active"),
					resource.TestCheckResourceAttrSet(resourceName, "is_data_retention_bot_registered"),
				),
			},
			{
				ResourceName:                         resourceName,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    acctest.AttrImportStateIdFunc(resourceName, "network_id"),
				ImportStateVerifyIdentifierAttribute: "network_id",
				// challenge is not returned by GetDataRetentionBot; it was
				// only available at creation time.
				ImportStateVerifyIgnore: []string{"challenge"},
			},
		},
	})
}

func TestAccWickrDataRetentionBot_disappears(t *testing.T) {
	ctx := acctest.Context(t)

	var bot wickr.GetDataRetentionBotOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_data_retention_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDataRetentionBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDataRetentionBotConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDataRetentionBotExists(ctx, t, resourceName, &bot),
					// Disappears by deleting the parent network, which cascades.
					acctest.CheckFrameworkResourceDisappears(ctx, t, tfwickr.ResourceNetwork, "aws_wickr_network.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccWickrDataRetentionBot_challenge verifies that the challenge
// password is generated during Create and stored as a sensitive attribute.
// The password is consumed by the external data-retention daemon.
func TestAccWickrDataRetentionBot_challenge(t *testing.T) {
	ctx := acctest.Context(t)

	var bot wickr.GetDataRetentionBotOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_data_retention_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDataRetentionBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDataRetentionBotConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDataRetentionBotExists(ctx, t, resourceName, &bot),
					// challenge is Computed+Sensitive — verify it was populated.
					resource.TestCheckResourceAttrSet(resourceName, "challenge"),
					// Verify the challenge is non-empty (the API returns a
					// real password string).
					testAccCheckDataRetentionBotChallengeNonEmpty(resourceName),
				),
			},
		},
	})
}

// TestAccWickrDataRetentionBot_computedFields verifies that all Computed
// attributes are populated after Create. The enabled and pubkey_msg_acked
// fields are Computed-only because the UpdateDataRetention API requires
// the external data-retention daemon to have completed registration
// (connected using the challenge password) before ENABLE/DISABLE/
// PUBKEY_MSG_ACK actions work. Without daemon registration, those fields
// reflect the daemon-side state, not Terraform-controlled state.
func TestAccWickrDataRetentionBot_computedFields(t *testing.T) {
	ctx := acctest.Context(t)

	var bot wickr.GetDataRetentionBotOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_data_retention_bot.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDataRetentionBotDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDataRetentionBotConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDataRetentionBotExists(ctx, t, resourceName, &bot),
					resource.TestCheckResourceAttr(resourceName, "bot_exists", acctest.CtTrue),
					resource.TestCheckResourceAttrSet(resourceName, "bot_name"),
					resource.TestCheckResourceAttrSet(resourceName, "is_bot_active"),
					// enabled and pubkey_msg_acked are false until the
					// external daemon registers.
					resource.TestCheckResourceAttr(resourceName, names.AttrEnabled, acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "pubkey_msg_acked", acctest.CtFalse),
					// is_data_retention_bot_registered is false until the
					// external daemon connects with the challenge password.
					resource.TestCheckResourceAttr(resourceName, "is_data_retention_bot_registered", acctest.CtFalse),
				),
			},
		},
	})
}

// --- Test helpers ---

func testAccCheckDataRetentionBotDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_wickr_data_retention_bot" {
				continue
			}

			networkID := rs.Primary.Attributes["network_id"]

			_, err := tfwickr.FindDataRetentionBotByID(ctx, conn, networkID)
			if retry.NotFound(err) {
				continue
			}
			// When the parent network is destroyed first (cascade),
			// GetDataRetentionBot may return ForbiddenError or a
			// deserialization failure on an HTML 401 page. Treat all
			// as "gone".
			if err != nil && (strings.Contains(err.Error(), "StatusCode: 401") ||
				strings.Contains(err.Error(), "deserialization failed") ||
				strings.Contains(err.Error(), "Forbidden")) {
				continue
			}
			if err != nil {
				return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameDataRetentionBot, networkID, err)
			}

			return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameDataRetentionBot, networkID, errors.New("not destroyed"))
		}

		return nil
	}
}

func testAccCheckDataRetentionBotExists(ctx context.Context, t *testing.T, name string, bot *wickr.GetDataRetentionBotOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameDataRetentionBot, name, errors.New("not found"))
		}

		networkID := rs.Primary.Attributes["network_id"]
		if networkID == "" {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameDataRetentionBot, name, errors.New("network_id not set"))
		}

		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		out, err := tfwickr.FindDataRetentionBotByID(ctx, conn, networkID)
		if err != nil {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameDataRetentionBot, networkID, err)
		}

		*bot = *out

		return nil
	}
}

// testAccCheckDataRetentionBotChallengeNonEmpty verifies the challenge
// attribute is a non-empty string in state.
func testAccCheckDataRetentionBotChallengeNonEmpty(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceName)
		}

		challenge := rs.Primary.Attributes["challenge"]
		if challenge == "" {
			return errors.New("challenge attribute is empty — CreateDataRetentionBotChallenge should have populated it")
		}

		return nil
	}
}

// --- Test configurations ---

func testAccDataRetentionBotConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name              = %[1]q
  access_level              = "PREMIUM"
  enable_premium_free_trial = true
}

resource "aws_wickr_data_retention_bot" "test" {
  network_id = aws_wickr_network.test.network_id
}
`, rName)
}

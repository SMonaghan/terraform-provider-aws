// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	tfwickr "github.com/hashicorp/terraform-provider-aws/internal/service/wickr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccWickrNetwork_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var network wickr.GetNetworkOutput
	// network_name is limited to 1-20 chars by the SDK, so use a fixed-length
	// 20-char random string rather than sdkacctest.RandomWithPrefix (which
	// yields 26 chars with a short prefix).
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkExists(ctx, t, resourceName, &network),
					resource.TestCheckResourceAttr(resourceName, "access_level", "STANDARD"),
					resource.TestCheckResourceAttr(resourceName, "network_name", rName),
					acctest.MatchResourceAttrRegionalARN(ctx, resourceName, names.AttrARN, "wickr", regexache.MustCompile(`network/[0-9]{8}$`)),
					resource.TestCheckResourceAttrSet(resourceName, "network_id"),
					resource.TestCheckResourceAttrSet(resourceName, names.AttrAWSAccountID),
				),
			},
			{
				ResourceName:                         resourceName,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    acctest.AttrImportStateIdFunc(resourceName, names.AttrARN),
				ImportStateVerifyIdentifierAttribute: names.AttrARN,
			},
		},
	})
}

func TestAccWickrNetwork_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var network wickr.GetNetworkOutput
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkExists(ctx, t, resourceName, &network),
					acctest.CheckFrameworkResourceDisappears(ctx, t, tfwickr.ResourceNetwork, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccWickrNetwork_networkName exercises the in-place update path for
// `network_name` — one of the two attributes UpdateNetwork accepts. The SDK
// caps the name at 1-20 chars; keep updated values within that range.
func TestAccWickrNetwork_networkName(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var network wickr.GetNetworkOutput
	rName1 := acctest.RandString(t, 20)
	rName2 := acctest.RandString(t, 20)
	resourceName := "aws_wickr_network.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckNetworkDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkConfig_basic(rName1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkExists(ctx, t, resourceName, &network),
					resource.TestCheckResourceAttr(resourceName, "network_name", rName1),
				),
			},
			{
				Config: testAccNetworkConfig_basic(rName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNetworkExists(ctx, t, resourceName, &network),
					resource.TestCheckResourceAttr(resourceName, "network_name", rName2),
				),
			},
		},
	})
}

// TestAccWickrNetwork_encryptionKeyArn intentionally omitted: the AWS Wickr
// service accepts `encryptionKeyArn` on CreateNetwork/UpdateNetwork but does
// not persist or echo it on GetNetwork (verified 2026-04-19, us-east-1, both
// STANDARD and PREMIUM tiers, no console option). See network.go schema
// comment. Reinstate this test when AWS ships end-to-end BYOK for Wickr
// networks.

func testAccCheckNetworkDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_wickr_network" {
				continue
			}

			networkID := rs.Primary.Attributes["network_id"]
			_, err := tfwickr.FindNetworkByID(ctx, conn, networkID)
			if retry.NotFound(err) {
				continue
			}
			// During the delete-in-progress window, GetNetwork returns
			// *awstypes.ForbiddenError rather than ResourceNotFoundError.
			// Treat it as "destroyed" for the purposes of this helper; the
			// production waiter in network.go already does the same.
			if errs.IsA[*awstypes.ForbiddenError](err) {
				continue
			}
			if err != nil {
				return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameNetwork, networkID, err)
			}

			return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameNetwork, networkID, errors.New("not destroyed"))
		}

		return nil
	}
}

func testAccCheckNetworkExists(ctx context.Context, t *testing.T, name string, network *wickr.GetNetworkOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameNetwork, name, errors.New("not found"))
		}

		networkID := rs.Primary.Attributes["network_id"]
		if networkID == "" {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameNetwork, name, errors.New("network_id not set"))
		}

		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		resp, err := tfwickr.FindNetworkByID(ctx, conn, networkID)
		if err != nil {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameNetwork, networkID, err)
		}

		*network = *resp

		return nil
	}
}

func testAccNetworkConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}
`, rName)
}

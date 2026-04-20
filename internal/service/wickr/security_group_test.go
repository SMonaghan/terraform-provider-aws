// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	tfwickr "github.com/hashicorp/terraform-provider-aws/internal/service/wickr"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccWickrSecurityGroup_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var sg awstypes.SecurityGroup
	// network_name is capped at 20 chars by the SDK; reuse the same value as
	// the SG name for clarity, which is well under the (undocumented) SG
	// name cap. See testAccNetworkConfig_basic in network_test.go.
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_security_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, rName),
					resource.TestCheckResourceAttr(resourceName, "is_default", acctest.CtFalse),
					resource.TestCheckResourceAttrSet(resourceName, "network_id"),
					resource.TestCheckResourceAttrSet(resourceName, "security_group_id"),
				),
			},
			{
				ResourceName:                         resourceName,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "network_id",
				ImportStateIdFunc:                    acctest.AttrsImportStateIdFunc(resourceName, ",", "network_id", "security_group_id"),
			},
		},
	})
}

func TestAccWickrSecurityGroup_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var sg awstypes.SecurityGroup
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_security_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					acctest.CheckFrameworkResourceDisappears(ctx, t, tfwickr.ResourceSecurityGroup, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccWickrSecurityGroup_name exercises the in-place update path for
// `name`. Update is expected, not replace (only `network_id` carries
// RequiresReplace).
func TestAccWickrSecurityGroup_name(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var sg awstypes.SecurityGroup
	rName1 := acctest.RandString(t, 20)
	rName2 := acctest.RandString(t, 20)
	resourceName := "aws_wickr_security_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupConfig_basic(rName1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, rName1),
				),
			},
			{
				Config: testAccSecurityGroupConfig_name(rName1, rName2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, rName2),
				),
			},
		},
	})
}

// TestAccWickrSecurityGroup_federationMode toggles the Create-settable
// `settings.federation_mode` knob. Valid *requestable* values are 1
// (Restricted) and 2 (Global) per the SDK's
// `SecurityGroupSettingsRequest` doc; the SDK's third documented value 0
// (Local) is not requestable — when submitted the service silently
// substitutes its default (1, Restricted) which trips a "Provider produced
// inconsistent result after apply" diagnostic. See task 6.6 Failure
// class C, and the accompanying int64validator.OneOf(1, 2) on the schema.
func TestAccWickrSecurityGroup_federationMode(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var sg awstypes.SecurityGroup
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_security_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupConfig_federationMode(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					resource.TestCheckResourceAttr(resourceName, "settings.0.federation_mode", "1"),
				),
			},
			{
				Config: testAccSecurityGroupConfig_federationMode(rName, 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					resource.TestCheckResourceAttr(resourceName, "settings.0.federation_mode", "2"),
				),
			},
		},
	})
}

// TestAccWickrSecurityGroup_lockoutThreshold toggles the Create-settable
// `settings.lockout_threshold` knob.
//
// The AWS Wickr service rejects `lockoutThreshold` values below the
// network-tier minimum ("invalid lockoutThreshold setting for network",
// HTTP 422). The observed minimum for STANDARD networks is 10, which
// matches the tier default; PREMIUM networks may allow lower values.
// This test toggles between two valid values (10 → 15) to avoid the
// tier floor. Users attempting values below the floor will see AWS's
// validation error surface through smarterr at apply time.
func TestAccWickrSecurityGroup_lockoutThreshold(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var sg awstypes.SecurityGroup
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_security_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupConfig_lockoutThreshold(rName, 10),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					resource.TestCheckResourceAttr(resourceName, "settings.0.lockout_threshold", "10"),
				),
			},
			{
				Config: testAccSecurityGroupConfig_lockoutThreshold(rName, 15),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					resource.TestCheckResourceAttr(resourceName, "settings.0.lockout_threshold", "15"),
				),
			},
		},
	})
}

// TestAccWickrSecurityGroup_settingsKitchenSink exercises every
// STANDARD-tier updatable leaf of the `settings` block in a single
// Create→Update→Destroy round-trip. Create sets every field to one
// value; Update flips every field to a different value; the Check
// steps assert the post-Apply state matches.
//
// Excluded fields and why:
//   - `calling` and `shredder` sub-blocks: `calling.*` is PREMIUM-only
//     per https://aws.amazon.com/wickr/pricing/, and `shredder` is
//     hard-blocked at the schema level due to an SDK/API shape gap
//     (see aws-sdk-go-v2-issue.md).
//   - `federation_mode = 0`: silently promoted to 1 by AWS on
//     STANDARD networks; the schema's int64validator.OneOf(1, 2)
//     blocks it at plan time.
//   - Every PREMIUM-only scalar (see `premiumOnlyFields` in
//     security_group.go): exercised by a separate PREMIUM-gated test
//     kept as a future TODO until a PREMIUM test network is available.
func TestAccWickrSecurityGroup_settingsKitchenSink(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var sg awstypes.SecurityGroup
	rName := acctest.RandString(t, 20)
	resourceName := "aws_wickr_security_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupConfig_kitchenSink(rName, kitchenSinkValuesA()),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					// Scalar bools (set A).
					resource.TestCheckResourceAttr(resourceName, "settings.0.enable_crash_reports", acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "settings.0.enable_restricted_global_federation", acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "settings.0.global_federation", acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "settings.0.is_link_preview_enabled", acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "settings.0.location_allow_maps", acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "settings.0.location_enabled", acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "settings.0.message_forwarding_enabled", acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "settings.0.presence_enabled", acctest.CtFalse),
					// Scalar ints (set A).
					resource.TestCheckResourceAttr(resourceName, "settings.0.federation_mode", "1"),
					resource.TestCheckResourceAttr(resourceName, "settings.0.lockout_threshold", "10"),
					// List-of-string.
					resource.TestCheckResourceAttr(resourceName, "settings.0.quick_responses.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "settings.0.quick_responses.0", "howdy"),
				),
			},
			{
				Config: testAccSecurityGroupConfig_kitchenSink(rName, kitchenSinkValuesB()),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSecurityGroupExists(ctx, t, resourceName, &sg),
					// Scalar bools (set B — every value flipped from A).
					resource.TestCheckResourceAttr(resourceName, "settings.0.enable_crash_reports", acctest.CtTrue),
					resource.TestCheckResourceAttr(resourceName, "settings.0.enable_restricted_global_federation", acctest.CtTrue),
					resource.TestCheckResourceAttr(resourceName, "settings.0.global_federation", acctest.CtTrue),
					resource.TestCheckResourceAttr(resourceName, "settings.0.is_link_preview_enabled", acctest.CtTrue),
					resource.TestCheckResourceAttr(resourceName, "settings.0.location_allow_maps", acctest.CtTrue),
					resource.TestCheckResourceAttr(resourceName, "settings.0.location_enabled", acctest.CtTrue),
					resource.TestCheckResourceAttr(resourceName, "settings.0.message_forwarding_enabled", acctest.CtTrue),
					resource.TestCheckResourceAttr(resourceName, "settings.0.presence_enabled", acctest.CtTrue),
					// Scalar ints (set B).
					resource.TestCheckResourceAttr(resourceName, "settings.0.federation_mode", "2"),
					resource.TestCheckResourceAttr(resourceName, "settings.0.lockout_threshold", "15"),
					// List-of-string (replaced).
					resource.TestCheckResourceAttr(resourceName, "settings.0.quick_responses.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "settings.0.quick_responses.0", "hi"),
					resource.TestCheckResourceAttr(resourceName, "settings.0.quick_responses.1", "bye"),
				),
			},
		},
	})
}

// TestAccWickrSecurityGroup_premiumPlanTierError verifies that setting
// a PREMIUM-only field on a STANDARD network produces the provider's
// enriched "feature requires a different level plan" error with the
// offending field named. Driven entirely against a STANDARD network
// (which is what the shared test runner creates), this is a
// negative-path test for the tier enforcement path.
func TestAccWickrSecurityGroup_premiumPlanTierError(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandString(t, 20)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.WickrServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSecurityGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccSecurityGroupConfig_alwaysReauthenticate(rName),
				ExpectError: regexache.MustCompile(`always_reauthenticate.*PREMIUM`),
			},
		},
	})
}

// Note: the `shredder` sub-block is hard-blocked at the schema level
// via `listvalidator.SizeAtMost(0)` until the upstream
// `aws-sdk-go-v2/service/wickr` type coverage for
// `types.ShredderSettings` matches the AWS API (the service requires
// `canProcessInBackground` and rejects Update requests that omit it;
// the Go SDK has no such field). See
// `.kiro/specs/aws-wickr-service/aws-sdk-go-v2-issue.md`. Once the
// SDK catches up, add a `TestAccWickrSecurityGroup_shredderIntensity`
// that exercises this Create-then-UpdateSecurityGroup path with
// intensity values from {0, 20, 60, 100}.

// Note: the default security group created implicitly by
// `CreateNetwork` is not manageable through `aws_wickr_security_group`.
// A dedicated `aws_wickr_default_security_group` resource will adopt
// the default SG (analogous to `aws_default_security_group` in the
// VPC service): Create adopts, Update mutates settings in place, and
// Delete is a silent state-only no-op. Until that resource ships,
// users should rely on AWS's automatic default-SG creation and avoid
// importing default SGs into this resource — AWS will reject any
// DeleteSecurityGroup call against a default SG with a clear error.

func testAccCheckSecurityGroupDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_wickr_security_group" {
				continue
			}

			networkID := rs.Primary.Attributes["network_id"]
			groupID := rs.Primary.Attributes["security_group_id"]

			// When the parent `aws_wickr_network` is destroyed first (the
			// common case for this test module, since a network delete
			// cascades), GetSecurityGroup against a gone network surfaces
			// one of several signals, all of which mean "SG is destroyed":
			//   - `*awstypes.ResourceNotFoundError` — the SG itself is gone
			//     (retry.NotFound catches this).
			//   - `*awstypes.ForbiddenError` — the parent network is in its
			//     delete-in-progress window (same pattern as the Network
			//     resource's Read handler; see security_group.go).
			//   - A smithy HTTP error whose message contains
			//     "StatusCode: 401" with a deserialization failure tail —
			//     this happens when the parent network has fully vanished
			//     and the Wickr API returns a non-JSON 401 page that the
			//     SDK decoder cannot parse. Treat as "gone".
			// Any other error still fails the test. See task 6.6 Failure
			// class E.
			_, err := tfwickr.FindSecurityGroupByID(ctx, conn, networkID, groupID)
			if retry.NotFound(err) {
				continue
			}
			if errs.IsA[*awstypes.ForbiddenError](err) ||
				errs.IsA[*awstypes.ResourceNotFoundError](err) ||
				(err != nil && strings.Contains(err.Error(), "StatusCode: 401")) {
				continue
			}
			if err != nil {
				return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameSecurityGroup, groupID, err)
			}

			return create.Error(names.Wickr, create.ErrActionCheckingDestroyed, tfwickr.ResNameSecurityGroup, groupID, errors.New("not destroyed"))
		}

		return nil
	}
}

func testAccCheckSecurityGroupExists(ctx context.Context, t *testing.T, name string, sg *awstypes.SecurityGroup) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameSecurityGroup, name, errors.New("not found"))
		}

		networkID := rs.Primary.Attributes["network_id"]
		groupID := rs.Primary.Attributes["security_group_id"]
		if networkID == "" || groupID == "" {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameSecurityGroup, name, errors.New("network_id or security_group_id not set"))
		}

		conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

		out, err := tfwickr.FindSecurityGroupByID(ctx, conn, networkID, groupID)
		if err != nil {
			return create.Error(names.Wickr, create.ErrActionCheckingExistence, tfwickr.ResNameSecurityGroup, groupID, err)
		}

		*sg = *out

		return nil
	}
}

func testAccSecurityGroupConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[1]q
}
`, rName)
}

func testAccSecurityGroupConfig_name(rNameNetwork, rNameSG string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[2]q
}
`, rNameNetwork, rNameSG)
}

func testAccSecurityGroupConfig_federationMode(rName string, mode int) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[1]q

  settings {
    federation_mode = %[2]d
  }
}
`, rName, mode)
}

func testAccSecurityGroupConfig_lockoutThreshold(rName string, threshold int) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[1]q

  settings {
    lockout_threshold = %[2]d
  }
}
`, rName, threshold)
}

// kitchenSinkValues collects every updatable `settings` leaf that works
// on a STANDARD-tier Wickr network. Two instances
// (kitchenSinkValuesA/B) drive the kitchen-sink test's Create and
// Update phases so every field flips between them.
//
// PREMIUM-only leaves (always_reauthenticate, check_for_updates,
// enable_atak, enable_file_download, enable_guest_federation,
// enable_notification_preview, enable_open_access_option,
// files_enabled, force_device_lockout, force_open_access,
// force_read_receipts, is_ato_enabled, max_auto_download_size,
// sso_max_idle_minutes, atak_package_values, the calling sub-block)
// are covered in a separate PREMIUM-gated test that this spec leaves
// TODO pending access to a PREMIUM test network. See the per-field
// tier matrix at https://aws.amazon.com/wickr/pricing/ (reproduced in
// `premiumOnlyFields` in security_group.go).
type kitchenSinkValues struct {
	// STANDARD-allowed scalars.
	EnableCrashReports               bool
	EnableRestrictedGlobalFederation bool
	GlobalFederation                 bool
	IsLinkPreviewEnabled             bool
	LocationAllowMaps                bool
	LocationEnabled                  bool
	MessageForwardingEnabled         bool
	PresenceEnabled                  bool
	FederationMode                   int
	LockoutThreshold                 int
	QuickResponses                   []string
}

func kitchenSinkValuesA() kitchenSinkValues {
	return kitchenSinkValues{
		EnableCrashReports:               false,
		EnableRestrictedGlobalFederation: false,
		GlobalFederation:                 false,
		IsLinkPreviewEnabled:             false,
		LocationAllowMaps:                false,
		LocationEnabled:                  false,
		MessageForwardingEnabled:         false,
		PresenceEnabled:                  false,
		FederationMode:                   1,
		LockoutThreshold:                 10,
		QuickResponses:                   []string{"howdy"},
	}
}

func kitchenSinkValuesB() kitchenSinkValues {
	return kitchenSinkValues{
		EnableCrashReports:               true,
		EnableRestrictedGlobalFederation: true,
		GlobalFederation:                 true,
		IsLinkPreviewEnabled:             true,
		LocationAllowMaps:                true,
		LocationEnabled:                  true,
		MessageForwardingEnabled:         true,
		PresenceEnabled:                  true,
		FederationMode:                   2,
		LockoutThreshold:                 15,
		QuickResponses:                   []string{"hi", "bye"},
	}
}

func hclStringList(vs []string) string {
	if len(vs) == 0 {
		return "[]"
	}
	quoted := make([]string, len(vs))
	for i, v := range vs {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func testAccSecurityGroupConfig_kitchenSink(rName string, v kitchenSinkValues) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[1]q

  settings {
    enable_crash_reports                = %[2]t
    enable_restricted_global_federation = %[3]t
    global_federation                   = %[4]t
    is_link_preview_enabled             = %[5]t
    location_allow_maps                 = %[6]t
    location_enabled                    = %[7]t
    message_forwarding_enabled          = %[8]t
    presence_enabled                    = %[9]t

    federation_mode   = %[10]d
    lockout_threshold = %[11]d

    quick_responses = %[12]s
  }
}
`,
		rName,
		v.EnableCrashReports, v.EnableRestrictedGlobalFederation, v.GlobalFederation,
		v.IsLinkPreviewEnabled, v.LocationAllowMaps, v.LocationEnabled,
		v.MessageForwardingEnabled, v.PresenceEnabled,
		v.FederationMode, v.LockoutThreshold,
		hclStringList(v.QuickResponses),
	)
}

func testAccSecurityGroupConfig_alwaysReauthenticate(rName string) string {
	return fmt.Sprintf(`
resource "aws_wickr_network" "test" {
  network_name = %[1]q
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = %[1]q

  settings {
    always_reauthenticate = true
  }
}
`, rName)
}

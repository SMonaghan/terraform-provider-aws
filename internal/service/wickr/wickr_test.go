// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/wickr"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/aws-sdk-go-base/v2/endpoints"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
)

// supportedRegions returns the Wickr_Supported_Regions set from the
// Glossary in the spec (see .kiro/specs/aws-wickr-service/requirements.md).
// Wickr is available in 11 commercial regions plus GovCloud us-gov-west-1;
// the provider's default test region (us-west-2) is NOT in this set.
func supportedRegions() []string {
	return []string{
		endpoints.UsEast1RegionID,
		endpoints.ApNortheast1RegionID,
		endpoints.ApSoutheast1RegionID,
		endpoints.ApSoutheast2RegionID,
		endpoints.ApSoutheast5RegionID,
		endpoints.CaCentral1RegionID,
		endpoints.EuCentral1RegionID,
		endpoints.EuCentral2RegionID,
		endpoints.EuWest2RegionID,
		endpoints.EuNorth1RegionID,
		endpoints.MeCentral1RegionID,
		endpoints.UsGovWest1RegionID,
	}
}

// testAccPreCheck is the shared PreCheck aggregator invoked by every Wickr
// acceptance test. It combines the provider-level PreCheck, the
// Wickr-region gate, and a live-API reachability probe (see
// testAccPreCheckAvailable below).
func testAccPreCheck(ctx context.Context, t *testing.T) {
	acctest.PreCheck(ctx, t)
	acctest.PreCheckRegion(t, supportedRegions()...)
	testAccPreCheckAvailable(ctx, t)
}

// testAccPreCheckAvailable probes ListNetworks once and skips (not
// fails) when the caller's principal lacks Wickr permissions.
func testAccPreCheckAvailable(ctx context.Context, t *testing.T) {
	conn := acctest.ProviderMeta(ctx, t).WickrClient(ctx)

	input := wickr.ListNetworksInput{}
	_, err := conn.ListNetworks(ctx, &input)

	// Implementation note for open question #5 (UnauthorizedError vs
	// ForbiddenError on a no-perms principal):
	// The SDK defines both `*awstypes.UnauthorizedError` and
	// `*awstypes.ForbiddenError`; without live API confirmation we match
	// both so the PreCheck doesn't false-fail in either regime. Whoever
	// runs the first live testacc pass should record the observed type
	// here and drop the unused branch. As of this writing the answer
	// remains unverified against live AWS.
	if errs.IsA[*awstypes.UnauthorizedError](err) ||
		errs.IsA[*awstypes.ForbiddenError](err) ||
		acctest.PreCheckSkipError(err) {
		t.Skipf("skipping acceptance testing: %s", err)
	}
	if err != nil {
		t.Fatalf("unexpected PreCheck error: %s", err)
	}
}

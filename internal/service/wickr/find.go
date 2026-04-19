// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

// findNetworkByID wraps GetNetwork with the provider's standard
// `*awstypes.ResourceNotFoundError` → `retry.NotFoundError` conversion
// (design.md → "Error handling — smarterr / `internal/smerr`"). The Wickr
// SDK surfaces its not-found sentinel as `*awstypes.ResourceNotFoundError`
// (note: "Error" suffix, not "Exception") — Requirement 2 item 20 verified.
//
// Implementation note for open question #4 (ResourceNotFoundError ErrorCode()):
// The type assertion via `errs.IsA[*awstypes.ResourceNotFoundError]` is the
// canonical signal; no secondary error-code-string check is needed. If a
// future caller wires `errs.IsAErrorMessageContains` for additional error
// classification, record the observed error-code string next to that check.
func findNetworkByID(ctx context.Context, conn *wickr.Client, id string) (*wickr.GetNetworkOutput, error) {
	input := wickr.GetNetworkInput{
		NetworkId: aws.String(id),
	}

	out, err := conn.GetNetwork(ctx, &input)
	if errs.IsA[*awstypes.ResourceNotFoundError](err) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}
	if err != nil {
		return nil, err
	}

	if out == nil || out.NetworkId == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return out, nil
}

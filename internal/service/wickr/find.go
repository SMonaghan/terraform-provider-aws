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

// findNetworkSettingsByID wraps GetNetworkSettings with the provider's
// standard `*awstypes.ResourceNotFoundError` → `retry.NotFoundError`
// conversion. The Wickr SDK surfaces its not-found sentinel as
// `*awstypes.ResourceNotFoundError`.
func findNetworkSettingsByID(ctx context.Context, conn *wickr.Client, networkID string) (*wickr.GetNetworkSettingsOutput, error) {
	input := wickr.GetNetworkSettingsInput{
		NetworkId: aws.String(networkID),
	}

	out, err := conn.GetNetworkSettings(ctx, &input)
	if errs.IsA[*awstypes.ResourceNotFoundError](err) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}
	if err != nil {
		return nil, err
	}

	if out == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return out, nil
}

// findSecurityGroupByID wraps GetSecurityGroup with the provider's standard
// `*awstypes.ResourceNotFoundError` → `retry.NotFoundError` conversion
// (design.md → "Error handling — smarterr / `internal/smerr`"). The SDK
// surfaces the not-found signal as `*awstypes.ResourceNotFoundError` (note
// the "Error" suffix, not "Exception").
func findSecurityGroupByID(ctx context.Context, conn *wickr.Client, networkID, groupID string) (*awstypes.SecurityGroup, error) {
	input := wickr.GetSecurityGroupInput{
		GroupId:   aws.String(groupID),
		NetworkId: aws.String(networkID),
	}

	out, err := conn.GetSecurityGroup(ctx, &input)
	if errs.IsA[*awstypes.ResourceNotFoundError](err) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}
	if err != nil {
		return nil, err
	}

	if out == nil || out.SecurityGroup == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return out.SecurityGroup, nil
}

// findBotByID wraps GetBot with the provider's standard
// `*awstypes.ResourceNotFoundError` → `retry.NotFoundError` conversion.
// The Wickr API returns HTTP 404 with `api error UnknownError: Bot not found`
// for deleted bots, which the SDK surfaces as a smithy error rather than
// `*awstypes.ResourceNotFoundError`. We check for both.
func findBotByID(ctx context.Context, conn *wickr.Client, networkID, botID string) (*wickr.GetBotOutput, error) {
	input := wickr.GetBotInput{
		BotId:     aws.String(botID),
		NetworkId: aws.String(networkID),
	}

	out, err := conn.GetBot(ctx, &input)
	if errs.IsA[*awstypes.ResourceNotFoundError](err) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}
	// The Wickr API returns HTTP 404 with "Bot not found" as an UnknownError
	// rather than ResourceNotFoundError for deleted bots (verified 2026-04-22,
	// us-east-1). Check the error message string as a fallback.
	if errs.Contains(err, "Bot not found") {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}
	if err != nil {
		return nil, err
	}

	if out == nil || out.BotId == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return out, nil
}

// findDataRetentionBotByID wraps GetDataRetentionBot with the provider's
// standard `*awstypes.ResourceNotFoundError` → `retry.NotFoundError`
// conversion. The data retention bot is a singleton per network; the only
// identifier is the network ID.
//
// GetDataRetentionBot does not return ResourceNotFoundError when the bot
// has not been created — it returns a response with BotExists=false. We
// treat BotExists==false as not-found so the resource lifecycle works
// correctly (Read removes from state, Delete treats as success).
func findDataRetentionBotByID(ctx context.Context, conn *wickr.Client, networkID string) (*wickr.GetDataRetentionBotOutput, error) {
	input := wickr.GetDataRetentionBotInput{
		NetworkId: aws.String(networkID),
	}

	out, err := conn.GetDataRetentionBot(ctx, &input)
	if errs.IsA[*awstypes.ResourceNotFoundError](err) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}
	if err != nil {
		return nil, err
	}

	if out == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	// The API returns a valid response even when no bot exists — check
	// the BotExists field to distinguish "bot provisioned" from "no bot".
	if out.BotExists != nil && !*out.BotExists {
		return nil, &retry.NotFoundError{
			LastError: tfresource.NewEmptyResultError(),
		}
	}

	return out, nil
}

// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wickr/types"
	"github.com/hashicorp/terraform-plugin-framework/types"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
)

// expandNetworkSettings converts the Terraform resource model into the
// structured `*awstypes.NetworkSettings` expected by
// `UpdateNetworkSettingsInput.Settings`.
//
// Only fields the user explicitly configured (non-null, non-unknown) are
// populated. The API accepts a sparse payload — omitted fields retain
// their current values server-side.
func expandNetworkSettings(model networkSettingsResourceModel) *awstypes.NetworkSettings {
	out := &awstypes.NetworkSettings{}
	hasAny := false

	if !model.EnableClientMetrics.IsNull() && !model.EnableClientMetrics.IsUnknown() {
		out.EnableClientMetrics = aws.Bool(model.EnableClientMetrics.ValueBool())
		hasAny = true
	}
	if !model.EnableTrustedDataFormat.IsNull() && !model.EnableTrustedDataFormat.IsUnknown() {
		out.EnableTrustedDataFormat = aws.Bool(model.EnableTrustedDataFormat.ValueBool())
		hasAny = true
	}

	if !model.ReadReceiptConfig.IsNull() && !model.ReadReceiptConfig.IsUnknown() {
		ctx := context.Background()
		if ptr, d := model.ReadReceiptConfig.ToPtr(ctx); d == nil || !d.HasError() {
			if ptr != nil && !ptr.Status.IsNull() && !ptr.Status.IsUnknown() {
				out.ReadReceiptConfig = &awstypes.ReadReceiptConfig{
					Status: awstypes.Status(ptr.Status.ValueString()),
				}
				hasAny = true
			}
		}
	}

	if !hasAny {
		return nil
	}

	return out
}

// flattenNetworkSettingsOutput materializes the flat `[]types.Setting` list
// returned by `GetNetworkSettingsOutput` into the structured Terraform
// resource model.
//
// Implementation note (open question #1 — Setting.OptionName string keys):
// The exact OptionName strings are inferred from the CamelCase field names
// on `types.NetworkSettings` (the Update-side structured type). The switch
// below handles the known keys; unrecognized keys are silently ignored so
// that new settings added by AWS do not break existing configurations.
//
// TODO: Verify the exact OptionName strings against a live
// GetNetworkSettings call in a Wickr-supported region and record the
// observed values here. As of this writing the answer remains unverified
// against live AWS.
func flattenNetworkSettingsOutput(ctx context.Context, out *wickr.GetNetworkSettingsOutput, model *networkSettingsResourceModel) {
	if out == nil {
		return
	}

	// Initialize all bool attributes to false. The API may not return
	// every setting in the flat list (e.g., a freshly created network
	// may omit settings that are at their default value). Terraform
	// requires all Computed attributes to have concrete values after
	// apply — leaving them Unknown causes "Provider returned invalid
	// result object after apply" errors.
	var dataRetention, enableClientMetrics, enableTrustedDataFormat bool
	var foundReadReceipt bool
	var readReceiptStatus string

	for _, setting := range out.Settings {
		name := aws.ToString(setting.OptionName)
		value := aws.ToString(setting.Value)

		switch strings.ToLower(name) {
		case "dataretention":
			dataRetention = parseBoolSetting(value)
		case "enableclientmetrics":
			enableClientMetrics = parseBoolSetting(value)
		case "enabletrusteddataformat":
			enableTrustedDataFormat = parseBoolSetting(value)
		default:
			// Check for read receipt status under various possible OptionName forms.
			lower := strings.ToLower(name)
			if lower == "readreceiptstatus" || lower == "readreceiptconfig.status" ||
				lower == "readreceipt" || strings.Contains(lower, "readreceipt") {
				foundReadReceipt = true
				readReceiptStatus = value
			}
			// Silently ignore unrecognized settings so that new AWS-side
			// settings do not break existing Terraform configurations.
		}
	}

	// Always set concrete values for all Computed bool attributes.
	model.DataRetention = types.BoolValue(dataRetention)
	model.EnableClientMetrics = types.BoolValue(enableClientMetrics)
	model.EnableTrustedDataFormat = types.BoolValue(enableTrustedDataFormat)

	// Populate read_receipt_config block if the API returned a value.
	if foundReadReceipt {
		rrc := &readReceiptConfigModel{
			Status: fwtypes.StringEnumValue(awstypes.Status(readReceiptStatus)),
		}
		rrcVal, d := fwtypes.NewListNestedObjectValueOfPtr(ctx, rrc)
		if d == nil || !d.HasError() {
			model.ReadReceiptConfig = rrcVal
		}
	}
}

// parseBoolSetting converts a string "true"/"false" (case-insensitive) to a
// Go bool. Returns false for any unrecognized value.
func parseBoolSetting(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}

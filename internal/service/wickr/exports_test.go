// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

// Exports for use in tests only.
var (
	ResourceNetwork          = newNetworkResource
	ResourceSecurityGroup    = newSecurityGroupResource
	ResourceNetworkSettings  = newNetworkSettingsResource
	ResourceBot              = newBotResource
	ResourceDataRetentionBot = newDataRetentionBotResource

	FindNetworkByID          = findNetworkByID
	FindSecurityGroupByID    = findSecurityGroupByID
	FindNetworkSettingsByID  = findNetworkSettingsByID
	FindBotByID              = findBotByID
	FindDataRetentionBotByID = findDataRetentionBotByID
)

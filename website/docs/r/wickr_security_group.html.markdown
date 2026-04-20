---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_security_group"
description: |-
  Manages an AWS Wickr security group within a network.
---

# Resource: aws_wickr_security_group

Manages an AWS Wickr security group. Security groups are children of a Wickr network and control the features and user-facing behavior available to their members. A network can have up to 100 security groups.

For more information, see the [AWS Wickr security group documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/edit-security-group.html).

~> **NOTE:** Every Wickr network is created with a default security group automatically. That default group is NOT manageable through `aws_wickr_security_group` — use a future `aws_wickr_default_security_group` resource to adopt it. Attempting to `terraform import` the default group into this resource and later `terraform destroy` will fail: the AWS Wickr API explicitly rejects `DeleteSecurityGroup` against the default group.

~> **NOTE:** Many settings fields require the `PREMIUM` network plan. Setting a PREMIUM-only field against a `STANDARD` network causes `terraform apply` to fail with an actionable provider error listing the offending fields. See [Plan tier requirements](#plan-tier-requirements) below and the [AWS Wickr pricing page](https://aws.amazon.com/wickr/pricing/) for the authoritative feature matrix.

## Example Usage

### Basic (Standard Plan)

```terraform
resource "aws_wickr_network" "example" {
  network_name = "example-network"
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "example" {
  network_id = aws_wickr_network.example.network_id
  name       = "engineering"
}
```

### With Settings (Standard-compatible)

```terraform
resource "aws_wickr_security_group" "engineering" {
  network_id = aws_wickr_network.example.network_id
  name       = "engineering"

  settings {
    federation_mode                     = 2 # Global
    global_federation                   = true
    enable_restricted_global_federation = true
    lockout_threshold                   = 10
    location_enabled                    = true
    location_allow_maps                 = true
    quick_responses                     = ["ACK", "Will do", "On it"]

    permitted_wickr_aws_networks {
      network_id = "12345678"
      region     = "us-east-1"
    }
  }
}
```

### Premium-only Features

```terraform
resource "aws_wickr_network" "example" {
  network_name = "example-premium"
  access_level = "PREMIUM"
}

resource "aws_wickr_security_group" "admins" {
  network_id = aws_wickr_network.example.network_id
  name       = "administrators"

  settings {
    # These require a PREMIUM network:
    always_reauthenticate = true
    is_ato_enabled        = true # Account-takeover protection
    max_auto_download_size = 10485760
    max_ttl                = 2592000 # 30 days
  }
}
```

## Argument Reference

The following arguments are required:

* `name` - (Required) Name of the security group.
* `network_id` - (Required, Forces new resource) Identifier of the parent `aws_wickr_network`. Changing this forces a new security group to be created in the target network.

The following arguments are optional:

* `settings` - (Optional) Nested block configuring the feature and policy controls applied to members of this security group. See [settings](#settings) below.
* `timeouts` - (Optional) Configuration block for operation timeouts. See [Timeouts](#timeouts).

### settings

Every field inside `settings` is optional. Fields omitted from HCL inherit the AWS server-side default for the network plan. Many fields require a `PREMIUM` network plan — see [Plan tier requirements](#plan-tier-requirements) for the full matrix.

Scalar fields:

* `always_reauthenticate` - (Optional, **PREMIUM**) Force users to re-authenticate every time they re-enter the application.
* `check_for_updates` - (Optional, **PREMIUM**) Automatically check for Wickr client updates on desktop.
* `enable_atak` - (Optional, **PREMIUM add-on**) Enable ATAK (Android Team Awareness Kit) integration. Requires a paid ATAK add-on.
* `enable_crash_reports` - (Optional) Allow clients to upload crash reports.
* `enable_file_download` - (Optional, **PREMIUM**) Allow users to download file attachments in their original form.
* `enable_guest_federation` - (Optional, **PREMIUM**) Allow members to communicate with guest users. Not available during the free trial.
* `enable_notification_preview` - (Optional, **PREMIUM**) Include a preview of message content in system notifications.
* `enable_open_access_option` - (Optional, **PREMIUM on STANDARD only as $5/user add-on**) Allow users to enable Wickr open access (traffic obfuscation).
* `enable_restricted_global_federation` - (Optional) Allow-list specific AWS Wickr or Wickr Enterprise networks that users can federate with. Requires `global_federation = true`; the provider rejects the combination at plan time.
* `federation_mode` - (Optional) Federation mode for the security group. Valid values: `1` (Restricted) and `2` (Global). Value `0` (Local) is silently promoted to `1` (Restricted) by the AWS API on non-PREMIUM networks and is not accepted in plan.
* `files_enabled` - (Optional, **PREMIUM**) Enable file attachments.
* `force_device_lockout` - (Optional, **PREMIUM**) Force device lockout threshold in minutes.
* `force_open_access` - (Optional, **PREMIUM**) Automatically enable Wickr open access on all devices.
* `force_read_receipts` - (Optional, **PREMIUM**) Force bot-delivered read receipts on.
* `global_federation` - (Optional) Allow federation with other Wickr networks regardless of region.
* `is_ato_enabled` - (Optional, **PREMIUM**) Enable account-takeover protection (two-factor enforcement on new device additions).
* `is_link_preview_enabled` - (Optional) Allow users to preview links inline.
* `location_allow_maps` - (Optional) Display a visual map for shared GPS coordinates.
* `location_enabled` - (Optional) Allow location sharing via GPS-enabled devices.
* `lockout_threshold` - (Optional) Number of failed attempts before lockout. The AWS Wickr API rejects values below a tier-specific minimum (observed as `10` for `STANDARD` networks) with HTTP 422.
* `max_auto_download_size` - (Optional, **PREMIUM**) Maximum image/file download size in bytes.
* `max_bor` - (Optional, **PREMIUM**) Maximum burn-on-read timer value (seconds). On `STANDARD` networks, the timer cap is fixed at 3 months.
* `max_ttl` - (Optional, **PREMIUM**) Maximum message expiration timer value (seconds). On `STANDARD` networks, the timer cap is fixed at 3 months; on `PREMIUM` the cap extends to 1 year.
* `message_forwarding_enabled` - (Optional) Allow users to forward messages.
* `permitted_networks` - (Optional) List of network identifiers that members may federate with (restricted federation).
* `presence_enabled` - (Optional) Display user presence (online/away/offline) indicators.
* `quick_responses` - (Optional) List of preset quick-response strings members can pick from.
* `show_master_recovery_key` - (Optional, **PREMIUM**) Display the master recovery key to users on account creation.
* `sso_max_idle_minutes` - (Optional, **PREMIUM**) Maximum idle time before SSO auto-destruct.

Nested blocks:

* `permitted_wickr_aws_networks` - (Optional) Allow-list of AWS Wickr networks that members may federate with. May be specified multiple times. See [permitted_wickr_aws_networks](#permitted_wickr_aws_networks).
* `permitted_wickr_enterprise_networks` - (Optional) Allow-list of Wickr Enterprise networks that members may federate with. May be specified multiple times. See [permitted_wickr_enterprise_networks](#permitted_wickr_enterprise_networks).

The `calling`, `password_requirements`, and `shredder` blocks are schema-declared but currently rejected at plan time; see [Known limitations](#known-limitations).

### permitted_wickr_aws_networks

* `network_id` - (Required) Identifier of the allow-listed AWS Wickr network.
* `region` - (Required) AWS region of the allow-listed network.

### permitted_wickr_enterprise_networks

* `domain` - (Required) Domain of the allow-listed Wickr Enterprise network.
* `network_id` - (Required) Identifier of the allow-listed Wickr Enterprise network.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `active_directory_guid` - Active Directory GUID associated with the security group, when AD sync is configured.
* `active_members` - Number of active human members in the security group.
* `bot_members` - Number of bot members in the security group.
* `is_default` - Whether this is the default security group for the network.
* `modified` - Epoch-second timestamp of the last modification.
* `security_group_id` - Unique identifier of the security group (shorthand alias: the SDK calls this `Id`).

## Plan tier requirements

The AWS Wickr service enforces tier-specific admin controls. Setting a PREMIUM-only field on a `STANDARD` network causes `terraform apply` to fail with an error listing the offending fields and a pointer to [https://aws.amazon.com/wickr/pricing/](https://aws.amazon.com/wickr/pricing/) (the authoritative feature matrix).

PREMIUM-only fields as of this writing:

* `always_reauthenticate`
* `check_for_updates`
* `enable_atak` (PREMIUM add-on)
* `enable_file_download`
* `enable_guest_federation`
* `enable_notification_preview`
* `enable_open_access_option` (PREMIUM, or STANDARD $5/user add-on)
* `files_enabled`
* `force_device_lockout`
* `force_open_access`
* `force_read_receipts`
* `is_ato_enabled`
* `max_auto_download_size`
* `max_bor`
* `max_ttl`
* `show_master_recovery_key`
* `sso_max_idle_minutes`
* `atak_package_values`
* `calling.can_start_11_call`, `calling.can_video_call`, `calling.force_tcp_call`

## Known limitations

* **`calling`, `password_requirements`, and `shredder` sub-blocks are currently blocked at plan time** (`listvalidator.SizeAtMost(0)`). The AWS API requires JSON fields that the upstream `github.com/aws/aws-sdk-go-v2/service/wickr` Go types do not include (`CALLING` uppercase key with 7 inner fields; `PasswordRequirements.regex`; `ShredderSettings.canProcessInBackground`), so the SDK cannot produce a request body the API accepts. This will be relaxed once the SDK catches up. Meanwhile, defaults for these settings are applied server-side at security-group creation and can be inspected (but not modified) via the corresponding computed attributes.
* **`federation_mode = 0` (Local)** is silently promoted to `1` (Restricted) by the AWS API on `STANDARD` networks. The schema rejects `0` at plan time via `int64validator.OneOf(1, 2)` to avoid a post-apply state drift.
* **Default security group** — destroying an `aws_wickr_security_group` that was imported from the default SG will fail with the raw API error. Use a dedicated `aws_wickr_default_security_group` resource (forthcoming) instead.

## Timeouts

[Configuration options](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts):

- `create` - (Default `30m`)
- `read` - (Default `10m`)
- `update` - (Default `30m`)
- `delete` - (Default `30m`)

## Import

In Terraform v1.12.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `identity` attribute. For example:

```terraform
import {
  to = aws_wickr_security_group.example
  identity = {
    "network_id"        = "12345678"
    "security_group_id" = "abcd1234"
  }
}

resource "aws_wickr_security_group" "example" {
  ### Configuration omitted for brevity ###
}
```

### Identity Schema

#### Required

- `network_id` (String) Identifier of the parent network.
- `security_group_id` (String) Identifier of the security group.

In Terraform v1.5.0 and later, use an [`import` block](https://developer.hashicorp.com/terraform/language/import) to import Wickr security groups using the comma-joined `network_id,security_group_id` pair. For example:

```terraform
import {
  to = aws_wickr_security_group.example
  id = "12345678,abcd1234"
}
```

Using `terraform import`, import Wickr security groups using the comma-joined identifier. For example:

```console
% terraform import aws_wickr_security_group.example 12345678,abcd1234
```

---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_security_group"
description: |-
  Provides details about an AWS Wickr security group within a network.
---

# Data Source: aws_wickr_security_group

Provides details about an AWS Wickr security group. Security groups are children of a Wickr network, identified by the `(network_id, security_group_id)` pair. See the [`aws_wickr_security_group` resource](../r/wickr_security_group.html.markdown) for the full management contract, including the settings surface and tier-enforcement rules.

~> **NOTE:** AWS Wickr is available only in specific regions. Attempting to read an `aws_wickr_security_group` data source in an unsupported region will return an endpoint error. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

## Example Usage

```terraform
data "aws_wickr_security_group" "example" {
  network_id        = "01234567"
  security_group_id = "abcd1234"
}
```

## Argument Reference

The following arguments are required:

* `network_id` - (Required) Identifier of the parent Wickr network.
* `security_group_id` - (Required) Identifier of the security group to look up.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `active_directory_guid` - Active Directory GUID associated with the security group, when AD sync is configured.
* `active_members` - Number of active human members in the security group.
* `bot_members` - Number of bot members in the security group.
* `is_default` - Whether this is the default security group for the network.
* `modified` - Epoch-second timestamp of the last modification.
* `name` - Name of the security group.
* `settings` - Nested block with the full security group settings surface (scalar feature flags, thresholds, timers, plus `calling`, `password_requirements`, `shredder`, `permitted_wickr_aws_networks`, and `permitted_wickr_enterprise_networks` sub-blocks). All fields on `settings` mirror the same-named arguments on the `aws_wickr_security_group` resource — see its [documentation](../r/wickr_security_group.html.markdown) for the full list and per-field tier requirements.

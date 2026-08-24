---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_network"
description: |-
  Provides details about an AWS Wickr network.
---

# Data Source: aws_wickr_network

Provides details about an AWS Wickr network. A Wickr network is the top-level organizational unit for AWS Wickr; it owns the security groups, bots, users, and other configuration associated with a deployment.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** AWS Wickr is available only in specific regions. Attempting to read an `aws_wickr_network` data source in an unsupported region will return an endpoint error. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

## Example Usage

```terraform
data "aws_wickr_network" "example" {
  network_id = "01234567"
}
```

## Argument Reference

The following arguments are required:

* `network_id` - (Required) Unique identifier of the Wickr network to look up.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `access_level` - Access level of the network. One of `STANDARD` or `PREMIUM`.
* `arn` - ARN of the network.
* `aws_account_id` - AWS account ID that owns the network.
* `enable_premium_free_trial` - Whether the network has a premium free trial enabled. This attribute may be null because `GetNetwork` does not return the value; the flag is only surfaced at Create time.
* `free_trial_expiration` - Expiration date and time for the network's free trial period, if applicable.
* `migration_state` - SSO redirect URI migration state managed by the SSO redirect migration wizard. Values: `0` (not started), `1` (in progress), or `2` (completed).
* `network_name` - Name of the network.
* `standing` - Current standing or status of the network.

---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_networks"
description: |-
  Lists all AWS Wickr networks in the caller's account and region.
---

# Data Source: aws_wickr_networks

Lists all AWS Wickr networks in the caller's account and region.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** AWS Wickr is available only in specific regions. Attempting to list `aws_wickr_networks` in an unsupported region will return an endpoint error. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

## Example Usage

```terraform
data "aws_wickr_networks" "example" {}
```

## Argument Reference

This data source does not support any arguments.

## Attribute Reference

This data source exports the following attributes:

* `networks` - List of Wickr networks. Each element contains:
    * `access_level` - Access level of the network. One of `STANDARD` or `PREMIUM`.
    * `arn` - ARN of the network.
    * `aws_account_id` - AWS account ID that owns the network.
    * `free_trial_expiration` - Expiration date and time for the network's free trial period, if applicable.
    * `migration_state` - SSO redirect URI migration state. Values: `0` (not started), `1` (in progress), or `2` (completed).
    * `network_id` - Unique identifier of the Wickr network.
    * `network_name` - Name of the network.
    * `standing` - Current standing or status of the network.

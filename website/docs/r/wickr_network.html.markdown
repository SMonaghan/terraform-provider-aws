---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_network"
description: |-
  Manages an AWS Wickr network.
---

# Resource: aws_wickr_network

Manages an AWS Wickr network. A Wickr network is the top-level organizational unit for AWS Wickr; it owns the security groups, bots, users, and other configuration associated with a deployment.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** AWS Wickr is available only in specific regions. Attempting to create an `aws_wickr_network` resource in an unsupported region will return an endpoint error. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

~> **NOTE:** `DeleteNetwork` cascades through all of the network's child resources (security groups, bots, users, network settings). Destroying an `aws_wickr_network` permanently removes all of that data.

## Example Usage

### Standard Network

```terraform
resource "aws_wickr_network" "example" {
  network_name = "example-network"
  access_level = "STANDARD"
}
```

### Premium Network with Free Trial

```terraform
resource "aws_wickr_network" "example" {
  network_name              = "example-premium"
  access_level              = "PREMIUM"
  enable_premium_free_trial = true
}
```

## Argument Reference

The following arguments are required:

* `access_level` - (Required, Forces new resource) Access level of the network. Valid values are `STANDARD` and `PREMIUM`.
* `network_name` - (Required) Name of the network. Must be between 1 and 20 characters.

The following arguments are optional:

* `enable_premium_free_trial` - (Optional, Forces new resource) Whether to enable a premium free trial for the network.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `arn` - ARN of the network.
* `aws_account_id` - AWS account ID that owns the network.
* `free_trial_expiration` - Expiration date and time for the network's free trial period, if applicable.
* `migration_state` - SSO redirect URI migration state managed by the SSO redirect migration wizard. Values: `0` (not started), `1` (in progress), or `2` (completed).
* `network_id` - Unique identifier of the network.
* `standing` - Current standing or status of the network.

## Timeouts

[Configuration options](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts):

- `create` - (Default `30m`)
- `read` - (Default `10m`)
- `update` - (Default `30m`)
- `delete` - (Default `60m`)

## Import

In Terraform v1.12.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `identity` attribute. For example:

```terraform
import {
  to = aws_wickr_network.example
  identity = {
    "arn" = "arn:aws:wickr:us-east-1:123456789012:network/0123456789abcdef0123456789abcdef"
  }
}

resource "aws_wickr_network" "example" {
  ### Configuration omitted for brevity ###
}
```

### Identity Schema

#### Required

- `arn` (String) ARN of the network.

In Terraform v1.5.0 and later, use an [`import` block](https://developer.hashicorp.com/terraform/language/import) to import Wickr networks using the `arn`. For example:

```terraform
import {
  to = aws_wickr_network.example
  id = "arn:aws:wickr:us-east-1:123456789012:network/0123456789abcdef0123456789abcdef"
}
```

Using `terraform import`, import Wickr networks using the `arn`. For example:

```console
% terraform import aws_wickr_network.example arn:aws:wickr:us-east-1:123456789012:network/0123456789abcdef0123456789abcdef
```

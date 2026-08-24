---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_data_retention_bot"
description: |-
  Manages the data retention bot for an AWS Wickr network.
---

# Resource: aws_wickr_data_retention_bot

Manages the singleton data retention bot for an AWS Wickr network. The data retention bot enables compliance logging by capturing messages for archival.

During creation, this resource automatically generates a challenge password via `CreateDataRetentionBotChallenge`. This password is used to configure the external data-retention daemon process. The password is stored in Terraform state as a sensitive attribute and is only available at creation time — subsequent reads do not return it.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** The `challenge` attribute contains a sensitive credential generated during resource creation. It is stored in Terraform state (encrypted at rest by your backend). To rotate the password, taint and recreate the resource.

~> **NOTE:** This resource requires a PREMIUM-tier Wickr network. STANDARD-tier networks do not support data retention bots.

~> **NOTE:** Destroying this resource calls `DeleteDataRetentionBot`, which removes the bot from AWS. Any external data-retention daemon configured with the challenge password will stop working. Recreating the resource generates a new bot with a new challenge password.

## Example Usage

### Basic Usage

```terraform
resource "aws_wickr_network" "example" {
  network_name              = "example-network"
  access_level              = "PREMIUM"
  enable_premium_free_trial = true
}

resource "aws_wickr_data_retention_bot" "example" {
  network_id = aws_wickr_network.example.network_id
}

# The challenge password for configuring the external data-retention daemon.
output "daemon_password" {
  value     = aws_wickr_data_retention_bot.example.challenge
  sensitive = true
}
```

## Argument Reference

The following arguments are required:

* `network_id` - (Required, Forces new resource) Unique identifier of the parent Wickr network. Must be a PREMIUM-tier network.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `challenge` - (Sensitive) The challenge password generated during resource creation. This password is used to configure the external data-retention daemon process. It is only available at creation time and is not returned by subsequent API reads. To rotate the password, taint and recreate the resource.
* `bot_exists` - Whether the data retention bot has been provisioned.
* `bot_name` - The automatically-assigned name of the data retention bot.
* `enabled` - Whether the data retention service is enabled. This value is controlled by the external data-retention daemon after it registers using the challenge password. It cannot be set directly through Terraform.
* `is_bot_active` - Whether the data retention bot is currently active.
* `is_data_retention_bot_registered` - Whether the data retention bot has registered itself against the data retention service. Becomes `true` after the external daemon connects using the challenge password.
* `pubkey_msg_acked` - Whether the public key message has been acknowledged by the external daemon.

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
  to = aws_wickr_data_retention_bot.example
  identity = {
    "network_id" = "12345678"
  }
}

resource "aws_wickr_data_retention_bot" "example" {
  ### Configuration omitted for brevity ###
}
```

### Identity Schema

#### Required

- `network_id` (String) Unique identifier of the parent Wickr network.

In Terraform v1.5.0 and later, use an [`import` block](https://developer.hashicorp.com/terraform/language/import) to import the Wickr data retention bot using the `network_id`. For example:

```terraform
import {
  to = aws_wickr_data_retention_bot.example
  id = "12345678"
}
```

Using `terraform import`, import the Wickr data retention bot using the `network_id`. For example:

```console
% terraform import aws_wickr_data_retention_bot.example 12345678
```

~> **NOTE:** The `challenge` attribute will not be populated after import because the API does not return the password. The password was only available at the time the resource was originally created.

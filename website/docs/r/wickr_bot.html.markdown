---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_bot"
description: |-
  Manages an AWS Wickr bot.
---

# Resource: aws_wickr_bot

Manages an AWS Wickr bot. A bot is an automated account within a Wickr network that can send and receive messages programmatically.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** AWS Wickr is available only in specific regions. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

## Example Usage

### Basic Bot

```terraform
resource "aws_wickr_network" "example" {
  network_name = "example-network"
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "example" {
  network_id = aws_wickr_network.example.network_id
  name       = "example-sg"
}

resource "aws_wickr_bot" "example" {
  network_id = aws_wickr_network.example.network_id
  group_id   = aws_wickr_security_group.example.security_group_id
  username   = "examplebot"
  challenge  = "my-secure-bot-password"
}
```

### Bot with Display Name

```terraform
resource "aws_wickr_bot" "example" {
  network_id   = aws_wickr_network.example.network_id
  group_id     = aws_wickr_security_group.example.security_group_id
  username     = "notifierbot"
  challenge    = "my-secure-bot-password"
  display_name = "Notification Bot"
}
```

## Argument Reference

The following arguments are required:

* `challenge` - (Required, **Sensitive**) Password for the bot. This value is user-supplied on both create and update (for password rotation). The password is never returned by the AWS API; only `has_challenge` indicates whether a password is set. This attribute is stored in the Terraform state file. Please ensure your state file is properly secured.
* `group_id` - (Required) Security group ID to assign the bot to. Can be updated in-place to move the bot between security groups.
* `network_id` - (Required, Forces new resource) Network ID of the Wickr network the bot belongs to.
* `username` - (Required, Forces new resource) Username for the bot. Must end in `bot` and be at least 4 characters long.

The following arguments are optional:

* `display_name` - (Optional) Display name for the bot.
* `suspend` - (Optional) Whether the bot is suspended. Defaults to the current API state.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `bot_id` - Unique identifier of the bot.
* `has_challenge` - Whether the bot has a password set.
* `last_login` - Timestamp of the bot's last login.
* `pubkey` - Bot's public encryption key.
* `status` - Current status of the bot. Values: `1` (pending), `2` (active).
* `suspended` - Whether the bot is currently suspended.
* `uname` - Username-hash identifier for the bot.

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
  to = aws_wickr_bot.example
  identity = {
    network_id = "12345678"
    bot_id     = "87654321"
  }
}
```

Using `terraform import`, import an AWS Wickr bot using the `network_id` and `bot_id` separated by a comma (`,`). For example:

```console
% terraform import aws_wickr_bot.example 12345678,87654321
```

~> **NOTE:** The `challenge` attribute cannot be imported. After import, you must set the `challenge` value in your configuration to match the bot's current password, or update it to a new password on the next apply.

---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_bot"
description: |-
  Provides details about an AWS Wickr bot.
---

# Data Source: aws_wickr_bot

Provides details about an AWS Wickr bot. A bot is an automated account within a Wickr network that can send and receive messages programmatically.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** AWS Wickr is available only in specific regions. Attempting to read an `aws_wickr_bot` data source in an unsupported region will return an endpoint error. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

## Example Usage

```terraform
data "aws_wickr_bot" "example" {
  network_id = "01234567"
  bot_id     = "87654321"
}
```

## Argument Reference

The following arguments are required:

* `bot_id` - (Required) Unique identifier of the bot to look up.
* `network_id` - (Required) Network ID of the Wickr network the bot belongs to.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `display_name` - Display name of the bot.
* `group_id` - Security group ID the bot is assigned to.
* `has_challenge` - Whether the bot has a password set.
* `last_login` - Timestamp of the bot's last login.
* `pubkey` - Bot's public encryption key.
* `status` - Current status of the bot. Values: `1` (pending), `2` (active).
* `suspended` - Whether the bot is currently suspended.
* `uname` - Username-hash identifier for the bot.
* `username` - Username of the bot.

~> **NOTE:** The bot password (`challenge`) is not available through this data source. The AWS API does not return the bot password; only `has_challenge` indicates whether a password is set.

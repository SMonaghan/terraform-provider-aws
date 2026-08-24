---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_bots"
description: |-
  Lists all bots in an AWS Wickr network.
---

# Data Source: aws_wickr_bots

Lists all bots in an AWS Wickr network. Optional filters allow narrowing results by display name, security group, status, or username.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** AWS Wickr is available only in specific regions. Attempting to read `aws_wickr_bots` in an unsupported region will return an endpoint error. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

## Example Usage

### List all bots in a network

```terraform
data "aws_wickr_bots" "example" {
  network_id = "01234567"
}
```

### Filter by security group

```terraform
data "aws_wickr_bots" "by_group" {
  network_id = "01234567"
  group_id   = "sg-abcdef"
}
```

## Argument Reference

The following arguments are required:

* `network_id` - (Required) Identifier of the parent Wickr network.

The following arguments are optional:

* `display_name` - (Optional) Filter bots by display name.
* `group_id` - (Optional) Filter bots by security group identifier.
* `status` - (Optional) Filter bots by status. `1` = pending, `2` = active.
* `username` - (Optional) Filter bots by username.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `bots` - List of bots in the network. Each element contains:
    * `bot_id` - Unique identifier of the bot.
    * `display_name` - Display name of the bot.
    * `group_id` - Security group identifier the bot belongs to.
    * `has_challenge` - Whether the bot has a challenge password set.
    * `last_login` - Timestamp of the bot's last login.
    * `pubkey` - Public key of the bot.
    * `status` - Status of the bot. `1` = pending, `2` = active.
    * `suspended` - Whether the bot is suspended.
    * `uname` - Internal username of the bot.
    * `username` - Username of the bot.

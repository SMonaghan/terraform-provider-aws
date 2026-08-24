---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_network_settings"
description: |-
  Provides details about the settings for an AWS Wickr network.
---

# Data Source: aws_wickr_network_settings

Provides details about the per-network settings for an AWS Wickr network. Network settings are a singleton child of a Wickr network — they are automatically created when the network is provisioned.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** AWS Wickr is available only in specific regions. Attempting to read an `aws_wickr_network_settings` data source in an unsupported region will return an endpoint error. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

## Example Usage

```terraform
data "aws_wickr_network_settings" "example" {
  network_id = "01234567"
}
```

## Argument Reference

The following arguments are required:

* `network_id` - (Required) Unique identifier of the parent Wickr network.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `data_retention` - Whether data retention is enabled for the network.
* `enable_client_metrics` - Whether client metrics collection is enabled.
* `enable_trusted_data_format` - Whether the trusted data format (OpenTDF) is enabled.
* `read_receipt_config` - Configuration block for read receipt behavior. See [`read_receipt_config`](#read_receipt_config) below.

### `read_receipt_config`

* `status` - Read receipt status. One of `DISABLED`, `ENABLED`, or `FORCE_ENABLED`.

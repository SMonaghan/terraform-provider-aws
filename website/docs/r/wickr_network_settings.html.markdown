---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_network_settings"
description: |-
  Manages the settings for an AWS Wickr network.
---

# Resource: aws_wickr_network_settings

Manages the per-network settings for an AWS Wickr network. Network settings are a singleton child of a Wickr network — they are automatically created when the network is provisioned and cannot be independently created or deleted.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** `terraform destroy` for this resource is a **state-only no-op**. The underlying network settings remain in AWS with their current values. There is no `DeleteNetworkSettings` API operation. To stop managing network settings with Terraform, remove the resource from your configuration or use `terraform state rm`. To reset settings to their defaults, update the resource's arguments to the desired values before removing it.

## Example Usage

### Basic Usage

```terraform
resource "aws_wickr_network" "example" {
  network_name = "example-network"
  access_level = "STANDARD"
}

resource "aws_wickr_network_settings" "example" {
  network_id = aws_wickr_network.example.network_id
}
```

### With All Settings

```terraform
resource "aws_wickr_network" "example" {
  network_name = "example-network"
  access_level = "STANDARD"
}

resource "aws_wickr_network_settings" "example" {
  network_id                 = aws_wickr_network.example.network_id
  data_retention             = true
  enable_client_metrics      = true
  enable_trusted_data_format = false

  read_receipt_config {
    status = "ENABLED"
  }
}
```

## Argument Reference

The following arguments are required:

* `network_id` - (Required, Forces new resource) Unique identifier of the parent Wickr network.

The following arguments are optional:

* `data_retention` - (Optional) Whether data retention is enabled for the network.
* `enable_client_metrics` - (Optional) Whether client metrics collection is enabled.
* `enable_trusted_data_format` - (Optional) Whether the trusted data format (OpenTDF) is enabled.
* `read_receipt_config` - (Optional) Configuration block for read receipt behavior. See [`read_receipt_config`](#read_receipt_config) below.

### `read_receipt_config`

* `status` - (Optional) Read receipt status. Valid values are `DISABLED`, `ENABLED`, and `FORCE_ENABLED`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

All arguments are also exported as attributes.

## Timeouts

[Configuration options](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts):

- `create` - (Default `10m`)
- `read` - (Default `5m`)
- `update` - (Default `10m`)
- `delete` - (Default `1m`)

## Import

In Terraform v1.12.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `identity` attribute. For example:

```terraform
import {
  to = aws_wickr_network_settings.example
  identity = {
    "network_id" = "12345678"
  }
}

resource "aws_wickr_network_settings" "example" {
  ### Configuration omitted for brevity ###
}
```

### Identity Schema

#### Required

- `network_id` (String) Unique identifier of the parent Wickr network.

In Terraform v1.5.0 and later, use an [`import` block](https://developer.hashicorp.com/terraform/language/import) to import Wickr network settings using the `network_id`. For example:

```terraform
import {
  to = aws_wickr_network_settings.example
  id = "12345678"
}
```

Using `terraform import`, import Wickr network settings using the `network_id`. For example:

```console
% terraform import aws_wickr_network_settings.example 12345678
```

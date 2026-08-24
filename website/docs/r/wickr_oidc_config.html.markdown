---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_oidc_config"
description: |-
  Manages the OIDC / SSO configuration for an AWS Wickr network.
---

# Resource: aws_wickr_oidc_config

Manages the OpenID Connect (OIDC) / Single Sign-On (SSO) configuration for an AWS Wickr network.

~> **NOTE:** Destroying this resource only removes it from Terraform state. The Wickr API does not support deleting OIDC configuration, so the underlying configuration will remain active on the network.

## Example Usage

### Basic OIDC Configuration

```terraform
resource "aws_wickr_network" "example" {
  network_name = "example-network"
  access_level = "STANDARD"
}

resource "aws_wickr_oidc_config" "example" {
  network_id = aws_wickr_network.example.network_id
  company_id = "UE1-ExampleCompany"
  issuer     = "https://login.microsoftonline.com/tenant-id/v2.0"
  scopes     = "openid profile email"
}
```

### With Pre-Validation

```terraform
resource "aws_wickr_oidc_config" "example" {
  network_id = aws_wickr_network.example.network_id
  company_id = "UE1-ExampleCompany"
  issuer     = "https://login.microsoftonline.com/tenant-id/v2.0"
  scopes     = "openid profile email"

  validate_before_save = true
}
```

## Argument Reference

The following arguments are required:

* `network_id` - (Required, Forces new resource) The ID of the Wickr network.
* `company_id` - (Required) Custom identifier your end users will use to sign in with SSO. Must include the Wickr region prefix (e.g., `UE1-` for `us-east-1`).
* `issuer` - (Required) The issuer URL of the OIDC provider.
* `scopes` - (Required) The OAuth scopes to request from the OIDC provider (e.g., `openid profile email`).

The following arguments are optional:

* `custom_username` - (Optional) A custom field mapping to extract the username from the OIDC token.
* `extra_auth_params` - (Optional) Additional authentication parameters to include in the OIDC flow.
* `secret` - (Optional, **Sensitive**) The client secret for authenticating with the OIDC provider.
* `sso_token_buffer_minutes` - (Optional) The buffer time in minutes before the SSO token expires to refresh it.
* `user_id` - (Optional) Unique identifier provided by your identity provider to authenticate the access request (also referred to as clientID).
* `validate_before_save` - (Optional) When `true`, calls the OIDC pre-validation endpoint before saving the configuration. Defaults to `false`. This is a provider-only flag and is not sent to the AWS API.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `application_id` - The unique identifier for the registered OIDC application.
* `application_name` - The name of the registered OIDC application.
* `ca_certificate` - The CA certificate used for secure communication with the OIDC provider.
* `client_id` - The OAuth client ID assigned to the application.
* `client_secret` - (**Sensitive**) The OAuth client secret for the application.
* `redirect_url` - The redirect URL configured for the OAuth flow.

## Timeouts

[Configuration options](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts):

* `create` - (Default `30m`)
* `read` - (Default `10m`)
* `update` - (Default `30m`)
* `delete` - (Default `30m`)

## Import

Wickr OIDC Config can be imported using the `network_id`:

```console
% terraform import aws_wickr_oidc_config.example network-id-value
```

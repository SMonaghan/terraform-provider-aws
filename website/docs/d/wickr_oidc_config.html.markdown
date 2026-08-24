---
subcategory: "Wickr"
layout: "aws"
page_title: "AWS: aws_wickr_oidc_config"
description: |-
  Provides details about the OIDC / SSO configuration for an AWS Wickr network.
---

# Data Source: aws_wickr_oidc_config

Provides details about the OpenID Connect (OIDC) / Single Sign-On (SSO) configuration for an AWS Wickr network.

For more information, see the [AWS Wickr documentation](https://docs.aws.amazon.com/wickr/latest/adminguide/what-is-wickr.html).

~> **NOTE:** AWS Wickr is available only in specific regions. Attempting to read an `aws_wickr_oidc_config` data source in an unsupported region will return an endpoint error. See the [Wickr endpoints page](https://docs.aws.amazon.com/general/latest/gr/wickr.html) for the current list.

## Example Usage

```terraform
data "aws_wickr_oidc_config" "example" {
  network_id = "01234567"
}
```

## Argument Reference

The following arguments are required:

* `network_id` - (Required) The ID of the Wickr network whose OIDC configuration to look up.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `application_id` - The unique identifier for the registered OIDC application.
* `application_name` - The name of the registered OIDC application.
* `ca_certificate` - The CA certificate used for secure communication with the OIDC provider.
* `client_id` - The OAuth client ID assigned to the application.
* `client_secret` - (**Sensitive**) The OAuth client secret for the application.
* `company_id` - Custom identifier end users use to sign in with SSO.
* `custom_username` - Custom field mapping to extract the username from the OIDC token.
* `extra_auth_params` - Additional authentication parameters included in the OIDC flow.
* `issuer` - The issuer URL of the OIDC provider.
* `redirect_url` - The redirect URL configured for the OAuth flow.
* `scopes` - The OAuth scopes requested from the OIDC provider.
* `secret` - (**Sensitive**) The client secret for authenticating with the OIDC provider.
* `sso_token_buffer_minutes` - The buffer time in minutes before the SSO token expires to refresh it.
* `user_id` - Unique identifier provided by the identity provider to authenticate the access request.

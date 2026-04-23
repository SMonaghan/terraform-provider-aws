# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "aws_wickr_network" "test" {
  region = var.region

  network_name = var.rName
  access_level = "STANDARD"
}

resource "aws_wickr_oidc_config" "test" {
  region = var.region

  network_id = aws_wickr_network.test.network_id
  company_id = "UE1-${var.rName}"
  issuer     = "https://login.microsoftonline.com/common/v2.0"
  scopes     = "openid profile email"
  user_id    = "00000000-0000-0000-0000-000000000000"
}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}

variable "region" {
  description = "Region to deploy resource in"
  type        = string
  nullable    = false
}

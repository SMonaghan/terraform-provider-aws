# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "aws_wickr_network" "test" {
  region = var.region

  network_name              = var.rName
  access_level              = "PREMIUM"
  enable_premium_free_trial = true
}

resource "aws_wickr_data_retention_bot" "test" {
  region = var.region

  network_id = aws_wickr_network.test.network_id
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

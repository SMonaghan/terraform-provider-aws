# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "aws_wickr_network" "test" {
  region = var.region

  network_name = var.rName
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  region = var.region

  network_id = aws_wickr_network.test.network_id
  name       = var.rName
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

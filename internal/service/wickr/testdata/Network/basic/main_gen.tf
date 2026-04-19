# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "aws_wickr_network" "test" {
  network_name = var.rName
  access_level = "STANDARD"
}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}

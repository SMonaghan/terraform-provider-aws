# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "aws_wickr_network" "test" {
  network_name = var.rName
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
  network_id = aws_wickr_network.test.network_id
  name       = var.rName
}

resource "aws_wickr_bot" "test" {
  network_id = aws_wickr_network.test.network_id
  group_id   = aws_wickr_security_group.test.security_group_id
  username   = "${var.rName}bot"
  challenge  = "test-challenge-pw-1"
}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}

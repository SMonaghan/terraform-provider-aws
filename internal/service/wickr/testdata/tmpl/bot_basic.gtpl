resource "aws_wickr_network" "test" {
{{- template "region" }}
  network_name = var.rName
  access_level = "STANDARD"
}

resource "aws_wickr_security_group" "test" {
{{- template "region" }}
  network_id = aws_wickr_network.test.network_id
  name       = var.rName
}

resource "aws_wickr_bot" "test" {
{{- template "region" }}
  network_id = aws_wickr_network.test.network_id
  group_id   = aws_wickr_security_group.test.security_group_id
  username   = "${var.rName}bot"
  challenge  = "test-challenge-pw-1"
}

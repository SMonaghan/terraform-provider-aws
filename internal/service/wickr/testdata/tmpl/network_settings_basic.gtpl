resource "aws_wickr_network" "test" {
{{- template "region" }}
  network_name = var.rName
  access_level = "STANDARD"
}

resource "aws_wickr_network_settings" "test" {
{{- template "region" }}
  network_id = aws_wickr_network.test.network_id
}

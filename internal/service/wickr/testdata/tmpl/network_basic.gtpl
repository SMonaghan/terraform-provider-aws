resource "aws_wickr_network" "test" {
{{- template "region" }}
  network_name = var.rName
  access_level = "STANDARD"
}

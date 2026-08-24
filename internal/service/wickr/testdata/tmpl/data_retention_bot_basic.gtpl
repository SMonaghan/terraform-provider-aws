resource "aws_wickr_network" "test" {
{{- template "region" }}
  network_name              = var.rName
  access_level              = "PREMIUM"
  enable_premium_free_trial = true
}

resource "aws_wickr_data_retention_bot" "test" {
{{- template "region" }}
  network_id = aws_wickr_network.test.network_id
}

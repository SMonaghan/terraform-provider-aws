resource "aws_wickr_network" "test" {
{{- template "region" }}
  network_name = var.rName
  access_level = "STANDARD"
}

resource "aws_wickr_oidc_config" "test" {
{{- template "region" }}
  network_id = aws_wickr_network.test.network_id
  company_id = "UE1-${var.rName}"
  issuer     = "https://login.microsoftonline.com/common/v2.0"
  scopes     = "openid profile email"
  user_id    = "00000000-0000-0000-0000-000000000000"
}

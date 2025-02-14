terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_dimension_value" "prod_env" {
  dimension_id = 12345
  name        = "Production"
  nrn         = "organization=1234567890:account=987654321:namespace=1122334455:value=prod"
}

resource "nullplatform_dimension_value" "staging_env" {
  dimension_id = 12345
  name        = "Staging"
  organization = "1234567890"
  account     = "my-account"
  namespace   = "platform-config"
}

resource "nullplatform_dimension_value" "dev_env" {
  dimension_id = data.nullplatform_dimension.env_dimension.id
  name        = "Development"
  nrn         = "${data.nullplatform_dimension.env_dimension.nrn}:value=dev"
}

output "prod_env_slug" {
  value = nullplatform_dimension_value.prod_env.slug
}

output "prod_env_status" {
  value = nullplatform_dimension_value.prod_env.status
}

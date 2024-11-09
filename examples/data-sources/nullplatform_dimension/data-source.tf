terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

data "nullplatform_dimension" "example" {
  id = "123456"
}

data "nullplatform_dimension" "by_nrn" {
  nrn = "organization=1234567890:account=987654321:namespace=1122334455"
}

data "nullplatform_dimension" "by_components" {
  organization = "1234567890"
  account     = "my-account"
  namespace   = "platform-config"
}

resource "nullplatform_dimension_value" "prod" {
  dimension_id = data.nullplatform_dimension.example.id
  name        = "Production"
  nrn         = "${data.nullplatform_dimension.example.nrn}:value=prod"
}

output "dimension_name" {
  value = data.nullplatform_dimension.example.name
}

output "dimension_slug" {
  value = data.nullplatform_dimension.example.slug
}

output "dimension_status" {
  value = data.nullplatform_dimension.example.status
}

output "dimension_order" {
  value = data.nullplatform_dimension.example.order
}
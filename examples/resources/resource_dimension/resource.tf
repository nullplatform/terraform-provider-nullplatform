terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_dimension" "ordered_dimension" {
  name  = "Region"
  order = 2
  nrn   = "organization=1234567890:account=987654321:namespace=1122334455"
}

resource "nullplatform_dimension" "component_dimension" {
  name      = "Department"
  account   = "my-main-account"
  namespace = "platform-config"
  order     = 3
}

output "dimension_slug" {
  description = "The generated slug for the dimension"
  value       = nullplatform_dimension.basic_dimension.slug
}

output "dimension_status" {
  description = "The current status of the dimension"
  value       = nullplatform_dimension.basic_dimension.status
}

terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

data "nullplatform_dimension_value" "existing_value" {
  dimension_id = 12345
  id          = "67890"
}

data "nullplatform_dimension_value" "by_nrn" {
  dimension_id = 12345
  nrn         = "organization=1234567890:account=987654321:namespace=1122334455:value=prod"
}

data "nullplatform_dimension_value" "by_components" {
  dimension_id  = 12345
  organization = "1234567890"
  account      = "my-account"
  namespace    = "platform-config"
  name         = "Production"
}

output "dimension_value_name" {
  value = data.nullplatform_dimension_value.existing_value.name
}

output "dimension_value_slug" {
  value = data.nullplatform_dimension_value.existing_value.slug
}

output "dimension_value_status" {
  value = data.nullplatform_dimension_value.existing_value.status
}

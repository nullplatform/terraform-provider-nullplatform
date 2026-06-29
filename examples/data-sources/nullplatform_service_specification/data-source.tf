terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Look up an existing service specification by its ID
data "nullplatform_service_specification" "example" {
  id = "4a4f6955-5ae0-40dc-a1de-e15e5cf41abb"
}

output "service_specification_name" {
  value = data.nullplatform_service_specification.example.name
}

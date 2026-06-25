terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "service_specification_id" {
  type        = string
  description = "The ID of the service specification to look up."
}

# Look up an existing service specification by its ID
data "nullplatform_service_specification" "example" {
  id = var.service_specification_id
}

output "service_specification_name" {
  value = data.nullplatform_service_specification.example.name
}

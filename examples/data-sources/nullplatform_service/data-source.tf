terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "service_id" {
  description = "ID of the service to look up."
  type        = string
}

# Look up an existing service by its ID.
data "nullplatform_service" "example" {
  id = var.service_id
}

output "service_name" {
  description = "Name of the resolved service."
  value       = data.nullplatform_service.example.name
}

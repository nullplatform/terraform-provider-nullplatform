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
  description = "ID of the service specification to list action specifications for."
}

# List all action specifications for the given service specification
data "nullplatform_action_specifications" "example" {
  service_specification_id = var.service_specification_id
}

output "action_specifications" {
  value = data.nullplatform_action_specifications.example.action_specifications
}

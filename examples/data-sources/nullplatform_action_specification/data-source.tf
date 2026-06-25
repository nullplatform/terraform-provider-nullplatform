terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "action_specification_id" {
  type        = string
  description = "The ID of the action specification to look up."
}

variable "service_specification_id" {
  type        = string
  description = "ID of the associated service specification."
}

# Look up an existing action specification by its ID and parent service specification
data "nullplatform_action_specification" "example" {
  id                       = var.action_specification_id
  service_specification_id = var.service_specification_id
}

output "action_specification_name" {
  value = data.nullplatform_action_specification.example.name
}

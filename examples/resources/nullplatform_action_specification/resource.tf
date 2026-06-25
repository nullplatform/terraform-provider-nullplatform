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
  description = "ID of the service specification this action belongs to"
}

# Resolve the parent service specification from its ID
data "nullplatform_service_specification" "this" {
  id = var.service_specification_id
}

resource "nullplatform_action_specification" "create_redis" {
  name                     = "Create Redis Instance"
  description              = "Provisions a Redis instance for the service"
  type                     = "create"
  service_specification_id = data.nullplatform_service_specification.this.id
  retryable                = true

  # Input parameters schema (JSON Schema) plus default values
  parameters = jsonencode({
    schema = {
      type = "object"
      properties = {
        size = {
          type    = "string"
          enum    = ["small", "medium", "large"]
          default = "small"
        }
      }
      required             = ["size"]
      additionalProperties = false
    }
    values = {
      size = "medium"
    }
  })

  # Expected outputs schema mapped to instance attributes
  results = jsonencode({
    schema = {
      type = "object"
      properties = {
        endpoint = { type = "string", target = "endpoint" }
        port     = { type = "number", target = "port" }
      }
      additionalProperties = false
    }
    values = {}
  })
}

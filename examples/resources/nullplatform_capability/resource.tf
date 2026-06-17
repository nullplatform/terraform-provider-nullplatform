terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_capability" "cpu_limits" {
  name        = "CPU Limits"
  target      = "scope"
  description = "Allowed CPU configurations for scopes"

  definition = jsonencode({
    type = "object"
    properties = {
      cpu = {
        type = "string"
        enum = ["0.25", "0.5", "1"]
      }
    }
    required             = ["cpu"]
    additionalProperties = false
  })
}

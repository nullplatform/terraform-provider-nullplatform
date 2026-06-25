terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application the deployment strategy applies to"
  type        = number
}

# Resolve the NRN from the target application instead of hardcoding it
data "nullplatform_application" "this" {
  id = var.application_id
}

resource "nullplatform_deployment_strategy" "rolling" {
  name        = "rolling-update"
  description = "Rolling update strategy for production scopes"
  nrn         = data.nullplatform_application.this.nrn

  # Restrict the strategy to a specific dimension
  dimensions = jsonencode({
    environment = "production"
  })

  # Strategy-specific tuning parameters
  parameters = jsonencode({
    max_unavailable = 1
    max_surge       = 1
  })
}

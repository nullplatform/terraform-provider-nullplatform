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
  description = "ID of the application the runtime configuration is scoped to."
  type        = number
}

# Resolve the application NRN from its ID
data "nullplatform_application" "app" {
  id = var.application_id
}

# Runtime configuration applied to the development environment dimension.
# Changing dimensions forces a new resource.
resource "nullplatform_runtime_configuration" "dev" {
  nrn = data.nullplatform_application.app.nrn

  dimensions = {
    environment = "dev"
  }

  # Settings that make up this runtime configuration
  values = {
    cpu    = "0.5"
    memory = "512"
  }
}

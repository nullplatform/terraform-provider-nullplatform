terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "nrn" {
  description = "The NRN the deployment strategy applies to"
  type        = string
}

resource "nullplatform_deployment_strategy" "rolling" {
  name        = "rolling-update"
  description = "Rolling update strategy for production scopes"
  nrn         = var.nrn

  dimensions = jsonencode({
    environment = "production"
  })

  parameters = jsonencode({
    max_unavailable = 1
    max_surge       = 1
  })

  scope_type_ids = []
}

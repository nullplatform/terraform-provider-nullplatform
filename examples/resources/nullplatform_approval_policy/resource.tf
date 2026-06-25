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
  description = "ID of the application the policy is scoped to."
  type        = number
}

data "nullplatform_application" "app" {
  id = var.application_id
}

# A policy expresses a MongoDB-style condition that must hold for an
# approval action to auto-approve.
resource "nullplatform_approval_policy" "min_coverage" {
  nrn  = data.nullplatform_application.app.nrn
  name = "Minimum test coverage 80%"

  conditions = jsonencode({
    "build.metadata.coverage.percentage" = { "$gte" = 80 }
  })
}

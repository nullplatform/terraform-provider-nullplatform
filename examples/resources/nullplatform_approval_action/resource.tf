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
  description = "ID of the application whose NRN scopes the approval action"
  type        = number
}

# Resolve the NRN from the application instead of hardcoding it
data "nullplatform_application" "this" {
  id = var.application_id
}

# Policy evaluated when this action is triggered
resource "nullplatform_approval_policy" "coverage" {
  nrn  = data.nullplatform_application.this.nrn
  name = "Code Coverage Policy - Minimum for production 80%"
  conditions = jsonencode({
    "build.metadata.coverage.percentage" = { "$gte" = 80 }
  })
}

resource "nullplatform_approval_action" "deployment_create" {
  nrn    = data.nullplatform_application.this.nrn
  entity = "deployment"
  action = "deployment:create"

  # Only require approval for the production environment
  dimensions = {
    environment = "production"
  }

  # Approve automatically when policies pass, fall back to manual review otherwise
  on_policy_success = "approve"
  on_policy_fail    = "manual"

  policies = [
    nullplatform_approval_policy.coverage.id
  ]
}

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
  description = "ID of the application the hook action is scoped to"
  type        = number
}

# Resolve the application's NRN instead of hardcoding it
data "nullplatform_application" "this" {
  id = var.application_id
}

resource "nullplatform_entity_hook_action" "scope_create" {
  nrn    = data.nullplatform_application.this.nrn
  entity = "scope"
  action = "scope:create"

  # Run before nullplatform's internal scope creation logic
  when = "before"
  on   = "create"
  type = "hook"

  # Restrict the hook to a specific set of dimensions
  dimensions = {
    environment = "production"
    country     = "us"
  }

  on_policy_success = "manual"
  on_policy_fail    = "manual"
}

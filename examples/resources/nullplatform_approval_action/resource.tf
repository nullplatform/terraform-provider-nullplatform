terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Policy evaluated by the actions below.
resource "nullplatform_approval_policy" "production" {
  nrn  = "organization=1:account=2:namespace=3:application=123"
  name = "Require approval in production"
  conditions = jsonencode({
    "context.dimensions.environment" = { "$eq" = "production" }
  })
}

# Require approval when a deployment is created in production.
resource "nullplatform_approval_action" "deployment_create" {
  nrn    = "organization=1:account=2:namespace=3:application=123"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail    = "manual"

  policies = [nullplatform_approval_policy.production.id]
}

# Require approval before deleting a scope in production.
resource "nullplatform_approval_action" "scope_delete" {
  nrn    = "organization=1:account=2:namespace=3:application=123"
  entity = "scope"
  action = "scope:delete"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail    = "manual"

  policies = [nullplatform_approval_policy.production.id]
}

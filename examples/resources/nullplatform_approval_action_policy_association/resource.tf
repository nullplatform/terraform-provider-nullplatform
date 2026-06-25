terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application the approval workflow is scoped to."
  type        = number
}

data "nullplatform_application" "app" {
  id = var.application_id
}

# Policy to evaluate when the action runs.
resource "nullplatform_approval_policy" "min_instances" {
  nrn  = data.nullplatform_application.app.nrn
  name = "Auto scaling - minimum 2 instances"
  conditions = jsonencode({
    "scope.capabilities.auto_scaling.enabled"              = true
    "scope.capabilities.auto_scaling.instances.min_amount" = { "$gte" = 2 }
  })
}

# Action the policy gates (creating a deployment).
resource "nullplatform_approval_action" "deployment_create" {
  nrn               = data.nullplatform_application.app.nrn
  entity            = "deployment"
  action            = "deployment:create"
  on_policy_success = "approve"
  on_policy_fail    = "manual"
}

# Associate the action with the policy so the policy is evaluated on the action.
resource "nullplatform_approval_action_policy_association" "example" {
  approval_action_id = nullplatform_approval_action.deployment_create.id
  approval_policy_id = nullplatform_approval_policy.min_instances.id
}

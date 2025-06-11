terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# First, create an approval policy
resource "nullplatform_approval_policy" "example" {
  nrn    = "organization=1:account=2:namespace=3:application=123"
  name   = "Auto Scaling Policy - Min Instances 2"
  conditions = jsonencode({
    "scope.capabilities.auto_scaling.enabled" = true,
    "scope.capabilities.auto_scaling.instances.min_amount" = 2
  })
}

# Then, create an approval action
resource "nullplatform_approval_action" "deployment_create" {
  nrn = "organization=1:account=2:namespace=3:application=123"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail = "manual"

  lifecycle {
    ignore_changes = [policies]
  }
}

# Finally, create the association between the action and policy
resource "nullplatform_approval_action_policy_association" "example" {
  approval_action_id  = nullplatform_approval_action.deployment_create.id
  approval_policy_id  = nullplatform_approval_policy.example.id
}

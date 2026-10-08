# The link block is deleted and the association is added, in one apply. The
# action keeps ignore_changes = [policies]: without it, the next plan would
# remove the policy the association adds.
resource "nullplatform_approval_action" "deployment_create" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail    = "manual"

  lifecycle {
    ignore_changes = [policies]
  }
}

resource "nullplatform_approval_action_policy_association" "coverage" {
  approval_action_id = nullplatform_approval_action.deployment_create.id
  approval_policy_id = nullplatform_approval_policy.coverage.id
}

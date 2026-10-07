# policies is removed from the action; the policy resources can stay.
resource "nullplatform_approval_action" "deployment_create" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail    = "manual"
}

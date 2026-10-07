# Unchanged: the action block stays as it was.
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

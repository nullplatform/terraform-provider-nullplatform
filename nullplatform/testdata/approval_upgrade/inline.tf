# An action with inline `policies`, as 0.0.105 supports it; inline.tfstate.json is the state 0.0.105
# left after applying it, with synthetic ids. The test declares and feeds var.nrn.
resource "nullplatform_approval_policy" "coverage" {
  nrn  = var.nrn
  name = "coverage"
  conditions = jsonencode({
    "build.metadata.coverage.code.coverage" = { "$gte" : 80 }
  })
}

resource "nullplatform_approval_action" "deployment_create" {
  nrn    = var.nrn
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  policies          = [nullplatform_approval_policy.coverage.id]
  on_policy_success = "approve"
  on_policy_fail    = "manual"
}

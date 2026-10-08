# An action that ignores `policies` and manages them with association resources, applied with 0.0.105
# (migrated.tfstate.json, synthetic ids) and then migrated to a
# checklist specification BY API; the .tf is left untouched. The test declares and feeds var.nrn.
resource "nullplatform_approval_policy" "coverage" {
  nrn  = var.nrn
  name = "coverage"
  conditions = jsonencode({
    "build.metadata.coverage.code.coverage" = { "$gte" : 80 }
  })
}

resource "nullplatform_approval_policy" "security" {
  nrn  = var.nrn
  name = "security"
  conditions = jsonencode({
    "build.metadata.security.security.vulnerabilities.critical" = { "$eq" : 0 }
  })
}

resource "nullplatform_approval_action" "deployment_create" {
  nrn    = var.nrn
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

resource "nullplatform_approval_action_policy_association" "security" {
  approval_action_id = nullplatform_approval_action.deployment_create.id
  approval_policy_id = nullplatform_approval_policy.security.id
}

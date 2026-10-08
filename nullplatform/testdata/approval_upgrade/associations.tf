# An action that ignores `policies` and manages them with association resources, one per policy:
# resource blocks as clients write them. associations.tfstate.json is the state 0.0.105 left after
# applying it, with synthetic ids. The test declares and feeds var.nrn_namespace.

#Policy to check that an app if a app is PCI. If not, the policy is approved.
resource "nullplatform_approval_policy" "PCI" {
  nrn  = var.nrn_namespace
  name = "PCI"
  conditions = jsonencode({
    "application.metadata.metadata_application.PCI" = "No"
  })
}

#Policy to check that new code has 80% of code coverage or more
resource "nullplatform_approval_policy" "coverage" {
  nrn  = var.nrn_namespace
  name = "Code Coverage"
  conditions = jsonencode({
    "build.metadata.coverage.code.coverage" = { "$gte" : 80 }
  })
}

#Policy to check that new code don´t have critical security vulnerabilities
resource "nullplatform_approval_policy" "security" {
  nrn  = var.nrn_namespace
  name = "Security"
  conditions = jsonencode({
    "build.metadata.security.security.vulnerabilities.critical" = { "$eq" : 0 }
  })
}

resource "nullplatform_approval_action" "deployment_create" {
  nrn    = var.nrn_namespace
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

resource "nullplatform_approval_action_policy_association" "PCI" {
  approval_action_id = nullplatform_approval_action.deployment_create.id
  approval_policy_id = nullplatform_approval_policy.PCI.id
}

resource "nullplatform_approval_policy" "respect_budget" {
  nrn  = var.nrn_namespace
  name = "Respect Budget"
  conditions = jsonencode({
    "application" : {
      "$expr" = {
        "$lte" = [
          "$application.metadata.finops.current_total_cost",
          "$application.metadata.finops.budget_assigned"
        ]
      }
    }
  })
}

resource "nullplatform_approval_action" "scope_create" {
  nrn    = var.nrn_namespace
  entity = "scope"
  action = "scope:create"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail    = "manual"
  lifecycle {
    ignore_changes = [
      policies
    ]
  }
  depends_on = [nullplatform_approval_policy.respect_budget]
}

resource "nullplatform_approval_action_policy_association" "respect_budget" {
  approval_action_id = nullplatform_approval_action.scope_create.id
  approval_policy_id = nullplatform_approval_policy.respect_budget.id

  depends_on = [nullplatform_approval_action.scope_create,
  nullplatform_approval_policy.respect_budget]
}

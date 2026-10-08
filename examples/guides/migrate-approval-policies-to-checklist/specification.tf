# The action had on_policy_success = "approve": requests that pass the
# conditions are approved with no reviewer, as they were with policies.
resource "nullplatform_checklist_specification" "deployment_review" {
  nrn         = "organization=1:account=2:namespace=3:application=4"
  name        = "deployment-review"
  description = "What deployment_create checked with policies"

  definition = jsonencode({
    execution_trigger = "automatic_approval"
    items = [
      {
        id       = "code_coverage"
        type     = "condition"
        behavior = "gate"
        title    = "Code coverage of at least 80%"
        query    = { "build.metadata.coverage.code.coverage" = { "$gte" = 80 } }
      }
    ]
  })
}

# The action had on_policy_success = "manual", the default: a reviewer signed
# off after the conditions passed. The last item keeps that sign-off.
resource "nullplatform_checklist_specification" "deployment_review" {
  nrn         = "organization=1:account=2:namespace=3:application=4"
  name        = "deployment-review"
  description = "What deployment_create checked with policies"

  definition = jsonencode({
    execution_trigger = "explicit"
    items = [
      {
        id       = "code_coverage"
        type     = "condition"
        behavior = "gate"
        title    = "Code coverage of at least 80%"
        query    = { "build.metadata.coverage.code.coverage" = { "$gte" = 80 } }
      },
      {
        id       = "human_sign_off"
        type     = "manual"
        behavior = "gate"
        title    = "A reviewer signs off"
      }
    ]
  })
}

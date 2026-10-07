terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Every change of definition or description creates a new version of the
# specification; current_version_id is the version an approval action links to.
resource "nullplatform_checklist_specification" "deployment_review" {
  nrn         = "organization=1:account=2:namespace=3:application=4"
  name        = "deployment-review"
  description = "A human signs off production deployments"

  definition = jsonencode({
    execution_trigger = "explicit"
    items = [
      {
        id       = "manual_review"
        type     = "manual"
        title    = "Manual review"
        behavior = "gate"
        severity = "major"
      }
    ]
  })
}

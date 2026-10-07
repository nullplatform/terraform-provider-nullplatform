terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_checklist_specification" "deployment_review" {
  nrn  = "organization=1:account=2:namespace=3:application=4"
  name = "deployment-review"

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

resource "nullplatform_approval_action" "deployment_create" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  on_checklist_fail = "manual"
}

# Follows the specification's current version: a new version re-links in place.
resource "nullplatform_approval_action_checklist_specification_association" "deployment_create" {
  approval_action_id         = nullplatform_approval_action.deployment_create.id
  checklist_specification_id = nullplatform_checklist_specification.deployment_review.current_version_id

  timeouts {
    create = "2m"
  }
}

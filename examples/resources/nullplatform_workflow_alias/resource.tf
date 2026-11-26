terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "checklist_nrn" {
  type        = string
  description = "The NRN whose checklist items start the workflow."
  default     = "organization=1:account=2"
}

resource "nullplatform_workflow" "release_notify" {
  name = "release-notify"
  key  = "release-notify"

  definition = jsonencode({
    steps = {
      start = {
        id         = "start"
        name       = "On release"
        pluginType = "webhook"
        config     = { path = "/release", mode = "start" }
      }
      announce = {
        id         = "announce"
        name       = "Announce"
        pluginType = "log"
        config     = { message = "A release was announced" }
      }
    }
    connections = [{ id = "c1", from = "start", to = "announce" }]
  })
}

# The alias follows the workflow's latest revision: every change of the
# definition is published and activated in the same apply.
resource "nullplatform_workflow_alias" "live" {
  workflow_id = nullplatform_workflow.release_notify.id
  name        = "live"
  revision    = nullplatform_workflow.release_notify.revision
}

# The URL ends in a token that is the capability to start the workflow, so it
# is sensitive. It is keyed by trigger id.
output "release_notify_webhook_url" {
  value     = nullplatform_workflow_alias.live.webhook_urls["start"]
  sensitive = true
}

# A workflow started by a checklist item: activating its alias registers the
# notification channel of the trigger at the NRN it names.
resource "nullplatform_workflow" "payments_gate" {
  name = "payments-gate"

  definition = jsonencode({
    steps = {
      gate = {
        id         = "gate"
        name       = "On checklist"
        pluginType = "np-checklist-trigger"
        config = {
          kind       = "payments-gate"
          nrn        = var.checklist_nrn
          mode       = "start"
          pathPrefix = "/np/checklist/payments-gate"
        }
      }
      say = {
        id         = "say"
        name       = "Say"
        pluginType = "log"
        config     = { message = "dispatched" }
      }
    }
    connections = [{ id = "c1", from = "gate", to = "say", sourcePort = "default", targetPort = "default" }]
  })
}

resource "nullplatform_workflow_alias" "payments_gate_live" {
  workflow_id = nullplatform_workflow.payments_gate.id
  name        = "live"
  revision    = nullplatform_workflow.payments_gate.revision
}

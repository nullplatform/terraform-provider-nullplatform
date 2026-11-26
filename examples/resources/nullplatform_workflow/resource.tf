terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The definition comes from a YAML file. Every change of it saves a new
# revision; the record's fields (name, key, description, path) are attributes,
# never written inside the definition.
resource "nullplatform_workflow" "release_notify" {
  name        = "release-notify"
  key         = "release-notify"
  path        = "/governance/releases"
  description = "Announces each release"

  definition = jsonencode(yamldecode(file("${path.module}/workflows/release-notify.yaml")))
}

# The same, with the definition written inline. Without a key the workflow is
# addressed by its id.
resource "nullplatform_workflow" "payments_gate" {
  name        = "payments-gate"
  description = "Logs a call to its webhook"

  definition = jsonencode({
    steps = {
      start = {
        id         = "start"
        name       = "On call"
        pluginType = "webhook"
        config     = { path = "/payments-gate", mode = "start" }
      }
      say = {
        id         = "say"
        name       = "Say"
        pluginType = "log"
        config     = { message = "called" }
      }
    }
    connections = [{ id = "c1", from = "start", to = "say" }]
  })
}

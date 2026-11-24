terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "account_nrn" {
  description = "NRN of the account that owns the application type, e.g. organization=1:account=2"
  type        = string
}

variable "namespace_id" {
  description = "ID of the namespace that owns the application"
  type        = number
}

# The schema is the form the platform shows when an application of this type
# is created, with the fields in the order written here. It is JSON in a
# heredoc (or file("agent.schema.json")) and not jsonencode(), which would
# put the keys of every object in alphabetical order.
resource "nullplatform_application_type" "agent" {
  nrn         = var.account_nrn
  name        = "AI Agent"
  description = "An agent that runs a model on a schedule or on demand"

  # Where the type can be used. Left out, it is the type's own NRN.
  visible_to = [var.account_nrn]

  schema = <<-JSON
    {
      "type": "object",
      "required": ["model"],
      "properties": {
        "model": {
          "type": "string",
          "title": "Model",
          "enum": ["small", "large"]
        },
        "autonomy": {
          "type": "string",
          "title": "Autonomy",
          "enum": ["low", "high"],
          "default": "low"
        },
        "max_steps": {
          "type": "integer",
          "title": "Maximum steps",
          "minimum": 1
        }
      }
    }
  JSON
}

# An application of the type: its attributes are checked against the schema.
resource "nullplatform_application" "support_bot" {
  name                = "support-bot"
  namespace_id        = var.namespace_id
  application_type_id = nullplatform_application_type.agent.id

  attributes = jsonencode({
    model     = "large"
    autonomy  = "low"
    max_steps = 20
  })
}

# A template that only applications of the type can start from.
resource "nullplatform_technology_template" "agent_starter" {
  name                = "Agent starter"
  url                 = "https://github.com/my-org/agent-starter"
  application_type_id = nullplatform_application_type.agent.id

  provider_config = {
    repository = "agent-starter"
  }

  components {
    type    = "language"
    id      = "python"
    version = "3.12"
  }
}

output "application_type_snapshot" {
  description = "The snapshot that holds the current schema; it changes when the schema does"
  value       = nullplatform_application_type.agent.last_snapshot_id
}

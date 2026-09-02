terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_api_key" "agent" {
  name     = "AGENT"
  internal = true

  grants {
    nrn       = "organization=1:account=1"
    role_slug = "controlplane:agent"
  }

  grants {
    nrn       = "organization=1:account=1"
    role_slug = "ops"
  }

  tags {
    key   = "managedBy"
    value = "IaC"
  }
}

output "agent_api_key_value" {
  value     = nullplatform_api_key.agent.api_key
  sensitive = true
}

terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# List all action specifications for the given service specification
data "nullplatform_action_specifications" "example" {
  service_specification_id = "4a4f6955-5ae0-40dc-a1de-e15e5cf41abb"
}

output "action_specifications" {
  value = data.nullplatform_action_specifications.example.action_specifications
}

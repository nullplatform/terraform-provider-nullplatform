terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Look up an existing action specification by its ID and parent service specification
data "nullplatform_action_specification" "example" {
  id                       = "123"
  service_specification_id = "4a4f6955-5ae0-40dc-a1de-e15e5cf41abb"
}

output "action_specification_name" {
  value = data.nullplatform_action_specification.example.name
}

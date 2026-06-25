terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "namespace_id" {
  description = "ID of the namespace that owns the application"
  type        = number
}

resource "nullplatform_application" "api" {
  name           = "my-api"
  namespace_id   = var.namespace_id
  repository_url = "https://github.com/my-org/my-api"

  tags = jsonencode({
    team = "platform"
  })

  settings = jsonencode({})
}

output "application_nrn" {
  value = nullplatform_application.api.nrn
}

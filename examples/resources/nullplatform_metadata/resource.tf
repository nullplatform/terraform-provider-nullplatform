terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  type        = number
  description = "ID of the application that holds the metadata"
}

# Look up the application to attach the metadata to
data "nullplatform_application" "this" {
  id = var.application_id
}

resource "nullplatform_metadata" "links" {
  entity    = "application"
  entity_id = data.nullplatform_application.this.id
  type      = "links"

  # JSON-encoded metadata value
  value = jsonencode([
    {
      title = "GitHub"
      icon  = "bi:github"
      links = [
        {
          url         = "https://github.com/my-organization"
          description = "Source code"
        }
      ]
    }
  ])
}

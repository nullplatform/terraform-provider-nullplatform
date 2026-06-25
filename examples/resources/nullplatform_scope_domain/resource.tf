terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "scope_id" {
  description = "ID of the scope the domain is attached to"
  type        = string
}

resource "nullplatform_scope_domain" "api" {
  name     = "api.example.com"
  scope_id = var.scope_id
  type     = "custom"
}

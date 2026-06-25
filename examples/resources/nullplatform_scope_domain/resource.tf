terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "scope_id" {
  description = "ID of the scope to attach the custom domain to"
  type        = string
}

# Resolve the scope so the domain references a real resource
data "nullplatform_scope" "this" {
  id = var.scope_id
}

resource "nullplatform_scope_domain" "api" {
  name     = "api.example.com"
  scope_id = data.nullplatform_scope.this.id
  type     = "custom"

  # Desired state of the domain attachment
  status = "active"
}

terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application whose NRN scopes the grant"
  type        = number
}

# Resolve the NRN from the application instead of hardcoding it
data "nullplatform_application" "target" {
  id = var.application_id
}

resource "nullplatform_user" "developer" {
  email      = "jane.doe@example.com"
  first_name = "Jane"
  last_name  = "Doe"
}

resource "nullplatform_authz_grant" "developer" {
  user_id   = nullplatform_user.developer.id
  role_slug = "application:developer"

  # Grant is scoped to the resolved application NRN
  nrn = data.nullplatform_application.target.nrn
}

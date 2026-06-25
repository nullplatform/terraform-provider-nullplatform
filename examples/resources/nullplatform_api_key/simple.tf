terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

# Application whose NRN scopes the API key grant
variable "simple_application_id" {
  type        = number
  description = "ID of the application to grant the API key access to."
}

data "nullplatform_application" "simple" {
  id = var.simple_application_id
}

resource "nullplatform_api_key" "simple" {
  name = "ci-deployer"

  # A single grant scoped to the application's NRN
  grants {
    nrn       = data.nullplatform_application.simple.nrn
    role_slug = "application:developer"
  }
}

terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

# Application whose NRN scopes the API key grants
variable "advanced_application_id" {
  type        = number
  description = "ID of the application to grant the API key access to."
}

data "nullplatform_application" "advanced" {
  id = var.advanced_application_id
}

resource "nullplatform_api_key" "advanced" {
  name = "platform-automation"

  # Multiple grants on the same application NRN with different roles
  grants {
    nrn       = data.nullplatform_application.advanced.nrn
    role_slug = "application:ops"
  }

  grants {
    nrn       = data.nullplatform_application.advanced.nrn
    role_slug = "application:developer"
  }

  # Optional tags for ownership and provenance
  tags {
    key   = "team"
    value = "platform"
  }

  tags {
    key   = "managed-by"
    value = "terraform"
  }
}

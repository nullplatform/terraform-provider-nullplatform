terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "null_application_id" {
  description = "Unique ID for the application the specification is scoped to"
  type        = number
}

# Resolve the application NRN to control specification visibility
data "nullplatform_application" "app" {
  id = var.null_application_id
}

resource "nullplatform_provider_specification" "aws" {
  name        = "AWS Configuration"
  description = "Settings for the AWS cloud provider integration"
  icon        = "aws"
  category    = "cloud-providers"

  # NRNs this specification is visible to
  visible_to = [
    data.nullplatform_application.app.nrn,
  ]

  # Allow per-dimension values and provide defaults
  allow_dimensions = true

  default_dimensions = jsonencode({
    environment = "production"
  })

  schema = jsonencode({
    type     = "object"
    required = ["region", "cluster"]
    properties = {
      region = {
        type        = "string"
        description = "AWS region"
      }
      cluster = {
        type        = "string"
        description = "EKS cluster name"
        tag         = true
      }
      access_key_id = {
        type        = "string"
        description = "AWS access key ID"
        secret      = true
      }
      secret_access_key = {
        type        = "string"
        description = "AWS secret access key"
        secret      = true
      }
    }
    additionalProperties = false
  })
}

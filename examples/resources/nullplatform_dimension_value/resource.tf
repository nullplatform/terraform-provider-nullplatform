terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "application_id" {
  description = "ID of the nullplatform application the dimension is scoped to."
  type        = number
}

data "nullplatform_application" "app" {
  id = var.application_id
}

# The parent dimension these values belong to.
resource "nullplatform_dimension" "environment" {
  nrn  = data.nullplatform_application.app.nrn
  name = "Environment"
}

# Each value is one option along the dimension (production, staging, ...).
resource "nullplatform_dimension_value" "production" {
  dimension_id = nullplatform_dimension.environment.id
  name         = "production"
  nrn          = data.nullplatform_application.app.nrn
}

resource "nullplatform_dimension_value" "staging" {
  dimension_id = nullplatform_dimension.environment.id
  name         = "staging"
  nrn          = data.nullplatform_application.app.nrn
}

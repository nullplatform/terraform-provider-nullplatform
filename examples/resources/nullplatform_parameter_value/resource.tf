terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application the parameter belongs to"
  type        = number
}

# Resolve the application NRN from its ID
data "nullplatform_application" "app" {
  id = var.application_id
}

# The parameter that owns the values defined below
resource "nullplatform_parameter" "log_level" {
  nrn      = data.nullplatform_application.app.nrn
  name     = "Log Level"
  variable = "LOG_LEVEL"
}

# Default value applied to every scope
resource "nullplatform_parameter_value" "default" {
  parameter_id = nullplatform_parameter.log_level.id
  nrn          = data.nullplatform_application.app.nrn
  value        = "INFO"
}

# Override the value for the development environment dimension
resource "nullplatform_parameter_value" "dev" {
  parameter_id = nullplatform_parameter.log_level.id
  nrn          = data.nullplatform_application.app.nrn
  value        = "DEBUG"
  dimensions = {
    environment = "dev"
  }
}

terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
      version = "~> 0.0.14"
    }
  }
}

variable "null_application_id" {
  description = "Unique ID for the application"
  type        = number
}

data "nullplatform_application" "app" {
  id = var.null_application_id
}

resource "nullplatform_parameter" "parameter" {
  nrn      = data.nullplatform_application.app.nrn
  name     = "Log Level"
  variable = "LOG_LEVEL"
}

resource "nullplatform_parameter_value" "any_scope_value" {
  parameter_id = nullplatform_parameter.parameter.id
  nrn          = data.nullplatform_application.app.nrn
  value        = "INFO"
}

resource "nullplatform_parameter_value" "env_value" {
  parameter_id = nullplatform_parameter.parameter.id
  nrn          = data.nullplatform_application.app.nrn
  value        = "DEBUG"
  dimensions   = { "environment": "dev" }
}

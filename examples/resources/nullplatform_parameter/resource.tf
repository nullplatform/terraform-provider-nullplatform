terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application the parameter belongs to."
  type        = number
}

data "nullplatform_application" "app" {
  id = var.application_id
}

# An environment parameter. It defines the variable; the actual values are
# set per scope/dimension with nullplatform_parameter_value.
resource "nullplatform_parameter" "log_level" {
  nrn      = data.nullplatform_application.app.nrn
  name     = "Log Level"
  variable = "LOG_LEVEL"
}

# A secret environment parameter (its value is stored encrypted).
resource "nullplatform_parameter" "api_token" {
  nrn      = data.nullplatform_application.app.nrn
  name     = "Third-party API token"
  variable = "API_TOKEN"
  secret   = true
}

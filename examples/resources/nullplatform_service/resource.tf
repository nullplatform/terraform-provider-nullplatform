terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "null_application_id" {
  description = "Unique ID for the application that owns the service"
  type        = number
}

variable "specification_id" {
  description = "Specification ID (UUID) for the service"
  type        = string
}

variable "open_weather_api_key" {
  description = "API key passed as a service attribute"
  type        = string
  sensitive   = true
}

data "nullplatform_application" "app" {
  id = var.null_application_id
}

# Declarative mode (import = true, the default): nullplatform records the
# service while provisioning is managed outside the platform.
resource "nullplatform_service" "open_weather" {
  name             = "open-weather"
  specification_id = var.specification_id
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]

  selectors {
    category     = "SaaS"
    imported     = true
    provider     = "OpenWeather"
    sub_category = "Weather"
  }

  attributes = {
    api_key = var.open_weather_api_key
  }
}

# Action-driven mode (import = false): the provider triggers the
# specification's create and delete actions to manage the infrastructure
# lifecycle.
resource "nullplatform_service" "open_weather_provisioned" {
  name             = "open-weather-provisioned"
  specification_id = var.specification_id
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]

  import = false

  selectors {
    category     = "SaaS"
    imported     = false
    provider     = "OpenWeather"
    sub_category = "Weather"
  }

  attributes = {
    api_key = var.open_weather_api_key
  }

  timeouts {
    create = "10m"
    delete = "10m"
  }
}

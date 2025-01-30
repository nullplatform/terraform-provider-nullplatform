terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "null_application_id" {
  description = "Unique ID for the application"
  type        = number
}

variable "open_weather_api_key" {
  description = "API Key for consume Open Weather services"
}

variable "specification_id" {
  description = "Specification ID for the service to be imported"
  type        = string
}
data "nullplatform_application" "app" {
  id = var.null_application_id
}

resource "nullplatform_service" "redis_cache_test" {
  name             = "redis-cache"
  specification_id = "4a4f6955-5ae0-40dc-a1de-e15e5cf41abb"
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]
  dimensions       = {}
  selectors = {
    imported = false,
  }
  attributes = {}
}

data "nullplatform_service" "service" {
  id = nullplatform_service.redis_cache_test.id
}

resource "nullplatform_service" "open_weather_test" {
  name             = "open-weather"
  specification_id = var.specification_id
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]
  selectors = {
    category     = "SaaS",
    imported     = true,
    provider     = "OpenWeather",
    sub_category = "Weather",
  }
  attributes = {
    api_key = var.open_weather_api_key
  }
  dimensions = {}
}

output "redis" {
  value = nullplatform_service.redis_cache_test
}

output "open_weather" {
  value = nullplatform_service.open_weather_test
}

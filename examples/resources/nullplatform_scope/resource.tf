terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "null_application_id" {
  description = "Unique ID for the application"
  type        = number
}

variable "environment" {
  description = "Environment name where the Scopes are deployed"
  default     = "dev"
}

locals {
  dimensions = {
    "environment" = lower(var.environment),
    "country"     = "arg"
  }
}

data "nullplatform_application" "app" {
  id = var.null_application_id
}

resource "nullplatform_scope" "example" {
  scope_name          = "${var.environment}-terraform-example-01"
  null_application_id = var.null_application_id

  # Lambda and log group NRN keys are configured with nullplatform_provider_config.
  capabilities_serverless_memory       = 512
  capabilities_serverless_handler_name = "thehandler"
  capabilities_serverless_runtime_id   = "java11"

  dimensions = local.dimensions
}

output "scope" {
  value = nullplatform_scope.example
}

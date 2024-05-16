terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
      version = "~> 0.0.14"
    }
  }
}

# Use the `NP_API_KEY` environment variable
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

  lambda_function_name            = "ScopeExample"
  lambda_current_function_version = "2"
  lambda_function_role            = "arn:aws:iam::300001300842:role/LambdaRole"
  lambda_function_main_alias      = upper(var.environment)
  lambda_function_warm_alias      = "WARM"

  capabilities_serverless_memory       = 512
  capabilities_serverless_handler_name = "thehandler"
  capabilities_serverless_runtime_id   = "java11"
  log_group_name                       = "/aws/lambda/ScopeExample"

  dimensions = local.dimensions
}

output "scope" {
  value = nullplatform_scope.test
}

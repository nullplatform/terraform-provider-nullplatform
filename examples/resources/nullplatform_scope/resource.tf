terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application that owns the scope."
  type        = number
}

# A serverless (AWS Lambda) scope.
#
# Note: the log_group_name and lambda_* attributes are deprecated but still
# required by the schema. The recommended approach is to configure those
# values through a nullplatform_provider_config resource instead.
resource "nullplatform_scope" "serverless" {
  scope_name          = "my-api-prod"
  null_application_id = var.application_id
  scope_type          = "serverless"

  # Serverless capabilities
  capabilities_serverless_handler_name     = "index.handler"
  capabilities_serverless_runtime_id       = "nodejs20.x"
  capabilities_serverless_runtime_platform = "x86_64"
  capabilities_serverless_memory           = 256
  capabilities_serverless_timeout          = 30

  # Deprecated but required by the schema (prefer nullplatform_provider_config)
  log_group_name                  = "/aws/lambda/my-api-prod"
  lambda_function_name            = "my-api-prod"
  lambda_current_function_version = "1"
  lambda_function_role            = "arn:aws:iam::123456789012:role/my-api-prod-exec"
  lambda_function_main_alias      = "active"

  # Restrict the scope to a set of dimensions
  dimensions = {
    environment = "production"
  }
}

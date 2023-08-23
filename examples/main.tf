terraform {
  required_providers {
    nullplatform= {
      version = "~> 0.0.4"
      source  = "hashicorp.com/com/nullplatform"
    }
  }
}

provider "nullplatform" {
  np_apikey = "NjA5ODc4MDU0.Z2tpMjk5QHhvb1BKckBxNFBzM0c1dDZIWFF3VjFENnc="
  np_api_url = "api.nullplatform.com"
}

resource "nullplatform_scope" "test" {
  scope_name = "terraform-test-9"
  null_application_id = 1558410685
  capabilities_serverless_handler_name = "thehandler"
  capabilities_serverless_runtime_id = "java11"
  s3_assets_bucket = "test-bucket"
  scope_workflow_role = "arn:aws:iam::300151377842:role/null-scope-and-deploy-manager"
  log_group_name = "/aws/lambda/test"
  lambda_function_name = "test"
  lambda_current_function_version = "1.0"
  lambda_function_role = "arn:aws:iam::300151377842:role/NP-ExchangeModelLambdas"
  lambda_function_main_alias = "REVIEW"
  log_reader_role = "arn:aws:iam::300151377842:role/null-telemetry-manager"
  lambda_function_warm_alias = "WARM"
}

resource "nullplatform_scope" "test1" {
  scope_name = "terraform-test-10"
  null_application_id = 1558410685
  capabilities_serverless_handler_name = "thehandler"
  capabilities_serverless_runtime_id = "java11"
  s3_assets_bucket = "test-bucket"
  scope_workflow_role = "arn:aws:iam::300151377842:role/null-scope-and-deploy-manager"
  log_group_name = "/aws/lambda/test"
  lambda_function_name = "test"
  lambda_current_function_version = "1.0"
  lambda_function_role = "arn:aws:iam::300151377842:role/NP-ExchangeModelLambdas"
  lambda_function_main_alias = "REVIEW"
  log_reader_role = "arn:aws:iam::300151377842:role/null-telemetry-manager"
  lambda_function_warm_alias = "WARM"
}

output "scope_var" {
  value = nullplatform_scope.test1.id
}

data "nullplatform_scope" "first" {
  id = nullplatform_scope.test1.id
}

output "first_order" {
  value = data.nullplatform_scope.first
}

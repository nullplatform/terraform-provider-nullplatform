terraform {
  required_providers {
    nullplatform= {
      version = "~> 0.0.2"
      source  = "hashicorp.com/com/nullplatform"
    }
  }
}

provider "nullplatform" {
  apikey = "NjA5ODc4MDU0.Z2tpMjk5QHhvb1BKckBxNFBzM0c1dDZIWFF3VjFENnc="
}

resource "nullplatform_scope" "test" {
  scope_name = "terraform-test-7"
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
  scope_name = "terraform-test-8"
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

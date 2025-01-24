terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_approval_policy" "example_with_account_name" {
  account = "kwik-e-mart-main"
  name    = "Example Approval Police-2"
  conditions = jsonencode({
    entity = "deployment"
    action = "create"
    rules = [
      {
        type     = "environment-2"
        operator = "in"
        values   = ["production"]
      }
    ]
  })
}

resource "nullplatform_approval_policy" "example" {
  nrn    = "organization=1255165411:account=95118862:namespace=1493172477:application=113444824"
  name   = "Example Approval Policy"
  conditions = jsonencode({
    entity = "deployment"
    action = "create"
    rules = [
      {
        type     = "environment-2"
        operator = "in"
        values   = ["production"]
      }
    ]
  })
}
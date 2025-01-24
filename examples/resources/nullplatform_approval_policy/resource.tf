terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_approval_policy" "example" {
 account = "test-account"
 name   = "Code Coverage Policy - Minimum for production 80%"
 conditions = jsonencode({
   "build.metadata.coverage.percentage" = { "$gte" = 80 }
 })
}

resource "nullplatform_approval_policy" "example" {
  nrn    = "organization=1:account=2:namespace=3:application=123"
  name   = "Auto Scaling Policy - Min Instances 2"
  conditions = jsonencode({
    "scope.capabilities.auto_scaling.enabled" = true,
    "scope.capabilities.auto_scaling.instances.min_amount" = 2
  })
}

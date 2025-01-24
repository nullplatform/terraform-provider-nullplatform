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
  nrn    = "organization=1255165411:account=95118862:namespace=1493172477:application=113444824"
  name   = "Auto Scaling Policy - Min Instances 2"
  conditions = jsonencode({
    "scope.capabilities.auto_scaling.enabled" = true,
    "scope.capabilities.auto_scaling.instances.min_amount" = 2
  })
}

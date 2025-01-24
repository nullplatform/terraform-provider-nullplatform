terraform {
  required_providers {
    nullplatform = {
        source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_approval_action" "deployment_create" {
    nrn = "organization=12551165411:account=2:namespace=3:application=123"
    entity = "deployment"
    action = "deployment:create"
    
    dimensions = {
      environment = "production"
    }

    on_policy_success = "approve"
    on_policy_fail = "manual"

    policies = [
      nullplatform_approval_policy.example_policy.id
    ]
}

resource "nullplatform_approval_action" "scope_delete" {
    nrn = "organization=12551165411:account=2:namespace=3:application=123"
    entity = "scope"
    action = "scope:delete"
    
    dimensions = {
        environment = "production"
    }

    on_policy_success = "approve"
    on_policy_fail = "manual"

    policies = [
      nullplatform_approval_policy.example_policy.id
    ]
}

resource "nullplatform_approval_action" "scope_create" {
    account = "test-account"
    entity = "scope"
    action = "scope:create"
    dimensions = {
      environment = "production"
    }
    on_policy_success = "approve"
    on_policy_fail = "manual"

    policies = [
      nullplatform_approval_policy.example_policy.id
    ]
}
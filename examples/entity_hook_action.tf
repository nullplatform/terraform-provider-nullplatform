terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Example 1: Entity hook action for scope creation before internal logic
resource "nullplatform_entity_hook_action" "scope_create_before" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "scope"
  action = "scope:create"

  dimensions = {
    environment = "production"
    country     = "us"
  }

  when = "before"
  type = "hook"
  on   = "create"

  on_policy_success = "manual"
  on_policy_fail    = "manual"
}

# Example 2: Entity hook action for deployment creation after internal logic
resource "nullplatform_entity_hook_action" "deployment_create_after" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  when = "after"
  type = "hook"
  on   = "create"

  on_policy_success = "manual"
  on_policy_fail    = "manual"
}

# Example 3: Entity hook action for application update using account slug
resource "nullplatform_entity_hook_action" "application_update" {
  account = "test-account"
  entity  = "application"
  action  = "application:write"

  when = "before"
  type = "hook"
  on   = "update"

  on_policy_success = "manual"
  on_policy_fail    = "manual"
}

# Example 4: Entity hook action for scope deletion
resource "nullplatform_entity_hook_action" "scope_delete" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "scope"
  action = "scope:delete"

  dimensions = {
    environment = "production"
  }

  when = "before"
  type = "hook"
  on   = "delete"

  on_policy_success = "manual"
  on_policy_fail    = "manual"
}

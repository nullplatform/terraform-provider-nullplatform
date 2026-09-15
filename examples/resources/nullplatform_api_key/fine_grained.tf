terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# A grant does not have to name an existing role. It can list the actions the
# key may call, or inherit existing roles and adjust the result — in both cases
# nullplatform creates a role private to this key, which you never manage.
#
# You can only grant actions you already hold at that NRN yourself.
resource "nullplatform_api_key" "ci" {
  name = "CI pipeline"

  # An existing role, as before.
  grants {
    nrn       = "organization=1:account=1"
    role_slug = "account:ops"
  }

  # Exactly these actions, and nothing else.
  grants {
    nrn     = "organization=1:account=1:namespace=1"
    actions = ["application:read", "deployment:create"]
  }

  # The union of two existing roles, plus one action, minus another. Listing
  # the two roles as separate grants instead would give the key two independent
  # roles, and there would be nothing to subtract from.
  grants {
    nrn            = "organization=1:account=1"
    inherits       = ["account:ops", "account:developer"]
    add_actions    = ["application:delete"]
    remove_actions = ["deployment:create"]
  }

  tags {
    key   = "terraform"
    value = "true"
  }
}

output "ci_api_key" {
  value     = nullplatform_api_key.ci.api_key
  sensitive = true
}

# What each grant actually resolves to, as reported by the API. Useful to
# confirm what an inherited grant ended up granting.
output "ci_effective_actions" {
  value = [for grant in nullplatform_api_key.ci.grants : grant.effective_actions]
}

---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "nullplatform_account Resource - nullplatform"
subcategory: ""
description: |-
  The account resource allows you to configure a nullplatform account
---

# nullplatform_account (Resource)

The account resource allows you to configure a nullplatform account

## Example Usage

```terraform
terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_account" "github_account" {
  name                = "My GitHub Account"
  repository_prefix   = "my-org"
  repository_provider = "github"
  slug                = "github-account"
}

resource "nullplatform_account" "gitlab_account" {
  name                = "My GitLab Account"
  repository_prefix   = "my-company"
  repository_provider = "gitlab"
  slug                = "gitlab-account"
}

output "github_account_id" {
  description = "The ID of the GitHub account"
  value       = nullplatform_account.github_account.id
}

output "github_account_org_id" {
  description = "The organization ID the account belongs to"
  value       = nullplatform_account.github_account.organization_id
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the account
- `repository_prefix` (String) The prefix used for repositories in this account
- `repository_provider` (String) The repository provider for this account
- `slug` (String) The unique slug identifier for the account

### Read-Only

- `id` (String) The ID of this resource.
- `organization_id` (Number) The ID of the organization this account belongs to (computed from authentication token)

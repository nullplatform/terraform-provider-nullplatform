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
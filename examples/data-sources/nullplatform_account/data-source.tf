terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

data "nullplatform_account" "existing_account" {
  id = "123456"
}

data "nullplatform_account" "by_slug" {
  slug = "github-account"
}

resource "nullplatform_namespace" "example" {
  name       = "Production Environment"
  account_id = data.nullplatform_account.existing_account.id
  
  depends_on = [
    data.nullplatform_account.existing_account
  ]
}

output "account_name" {
  description = "The name of the account"
  value       = data.nullplatform_account.existing_account.name
}

output "account_repo_prefix" {
  description = "The repository prefix of the account"
  value       = data.nullplatform_account.existing_account.repository_prefix
}

output "account_repo_provider" {
  description = "The repository provider of the account"
  value       = data.nullplatform_account.existing_account.repository_provider
}

resource "nullplatform_account" "multi_provider" {
  for_each = {
    github = {
      name      = "GitHub Projects"
      prefix    = "github-org"
      provider  = "github"
      slug      = "github-projects"
    }
    gitlab = {
      name      = "GitLab Projects"
      prefix    = "gitlab-org"
      provider  = "gitlab"
      slug      = "gitlab-projects"
    }
  }

  name                = each.value.name
  repository_prefix   = each.value.prefix
  repository_provider = each.value.provider
  slug                = each.value.slug
}

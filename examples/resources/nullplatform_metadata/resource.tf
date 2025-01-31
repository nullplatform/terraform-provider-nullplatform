terraform {
  required_providers {
    nullplatform = {
      version = "0.0.14"
      source  = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_metadata" "application_links" {
  entity    = "application"
  entity_id = "179769363"
  type      = "links"
  
  value = jsonencode([
    {
      title = "Trello"
      icon  = "logos:trello"
      links = [
        {
          url         = "https://trello.com/w/my-organization-trello/home"
          description = "Workspace"
        }
      ]
    },
    {
      title = "Github"
      icon  = "bi:github"
      links = [
        {
          url         = "https://github.com/my-organization-github"
          description = "Homepage"
        }
      ]
    }
  ])
}

# Example: Application Frameworks
# This example demonstrates how to specify the frameworks used in an application
# This is a custom metadata type
resource "nullplatform_metadata" "application_frameworks" {
  entity    = "application"
  entity_id = "179769363"
  type      = "frameworks"
  
  value = jsonencode({
    backend = [
      {
        name    = "Spring Boot"
        version = "3.2.0"
      },
      {
        name    = "Java"
        version = "17"
      }
    ],
    frontend = [
      {
        name    = "React"
        version = "18.2.0"
      },
      {
        name    = "TypeScript"
        version = "5.0.0"
      }
    ]
  })
}

# Example: Application Coverage
# This example shows how to manage code coverage metadata
# This is a custom metadata type
resource "nullplatform_metadata" "application_coverage" {
  entity    = "application"
  entity_id = "179769363"
  type      = "coverage"
  
  value = jsonencode({
    overall = 85.6,
    components = {
      backend = {
        percentage = 92.3,
        details = {
          lines      = 90.5,
          branches   = 85.2,
          functions  = 95.1,
          statements = 93.4
        }
      },
      frontend = {
        percentage = 78.9,
        details = {
          lines      = 80.2,
          branches   = 75.6,
          functions  = 82.3,
          statements = 77.5
        }
      }
    }
  })
}

terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# To enable slack integration please refer first to https://docs.nullplatform.com/docs/notifications/#slack

# Slack channels will not be valid for organization NRNs. They must be in a lower level, at least account level.
resource "nullplatform_notification_channel" "slack" {
 nrn    = "organization=1:account=2:namespace=3:application=123"
 type   = "slack"
 source = ["approval"]

 configuration {
   slack {
     channels = ["alerts", "platform-notifications"] # Multiple channels can be specified
   }
 }
}

resource "nullplatform_notification_channel" "webhook" {
 nrn  = "organization=1:account=2:namespace=3:application=123"
 type = "http"
 source = ["approval"]

 configuration {
   http {
     url = "https://hooks.example.com/webhook/xyz" # Custom webhook URL - can contain headers
     headers = {
        "Auhorization" = "Bearer xyz"
     }
   }
 }
}

resource "nullplatform_notification_channel" "github" {
 nrn    = "organization=1:account=2:namespace=3:application=123"
 type   = "github"
 source = ["service"]

 configuration {
   github {
     account         = "my-github-org"
     reference       = "main"
     repository      = "my-awesome-repo"
     workflow_id     = "provisioning.yml"
     installation_id = "12345678"
   }
 }
}

resource "nullplatform_notification_channel" "agent" {
  nrn    = "organization=1:account=2:namespace=3:application=123"
  type   = "agent"
  source = ["service"]

  configuration {
    agent {
      api_key     = "my-agent-api-key"
      command {
        data = {
          cmdline = "get-current-connections '$${NOTIFICATION_CONTEXT}'"
        }
        type = "exec"
      }
      selector = {
        environment = "dev"
      }
    }
  }
}

output "slack_channel_source" {
 value = nullplatform_notification_channel.slack.source
}

output "webhook_channel_url" {
 value = nullplatform_notification_channel.webhook.configuration[0].http[0].url
}

output "github_channel_repository" {
 value = nullplatform_notification_channel.github.configuration[0].github[0].repository
}

output "agent_channel_command" {
 value = nullplatform_notification_channel.custom_agent.configuration[0].agent[0].command[0].data.cmdline
}

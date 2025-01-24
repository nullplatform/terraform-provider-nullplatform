terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_notification_channel" "slack" {
 nrn    = "organization=1255165411:account=95118862:namespace=1493172477:application=113444824"
 type   = "slack"
 source = ["approval"]
 
 configuration {
   slack {
     channels = ["alerts", "platform-notifications"] # Multiple channels can be specified
   }
 }
}

resource "nullplatform_notification_channel" "webhook" {
 nrn  = "organization=1255165411:account=95118862:namespace=1493172477:application=113444824"
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
 nrn    = "organization=1255165411:account=95118862:namespace=1493172477:application=113444824"
 type   = "github"
 source = ["service"]
 
 configuration {
   github {
     account         = "GalactusPlatform"
     reference       = "main"
     repository      = "demo-nullplatform-services-manager"
     workflow_id     = "service-action.yml"
     installation_id = "57594772"
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
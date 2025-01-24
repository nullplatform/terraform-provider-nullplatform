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
  source = ["approval", "service"]
  
  configuration {
    slack {
      channels = ["#alerts", "#platform-notifications"]
    }
  }
}

resource "nullplatform_notification_channel" "webhook" {
  nrn  = "organization=1255165411:account=95118862:namespace=1493172477:application=113444824"
  type = "http"
  source = ["approval", "service"]
  
  configuration {
    http {
      url = "https://hooks.example.com/webhook/xyz"
    }
  }
}

resource "nullplatform_notification_channel" "github" {
  nrn    = "organization=1255165411:account=95118862:namespace=1493172477:application=113444824"
  type   = "github"
  source = ["approval", "service"]
  
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
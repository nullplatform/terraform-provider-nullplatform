terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

# The application whose NRN the notification channel will be scoped to.
# Slack channels must be set at account level or lower (not at organization level).
variable "application_id" {
  type        = number
  description = "ID of the application to attach the notification channel to"
}

data "nullplatform_application" "this" {
  id = var.application_id
}

# Notify a Slack channel when an approval is requested.
# See https://docs.nullplatform.com/docs/notifications/#slack to enable the integration first.
resource "nullplatform_notification_channel" "slack" {
  nrn         = data.nullplatform_application.this.nrn
  type        = "slack"
  source      = ["approval"]
  description = "Slack notifications for approvals"

  configuration {
    slack {
      channels = ["alerts", "platform-notifications"] # One or more Slack channels
    }
  }
}

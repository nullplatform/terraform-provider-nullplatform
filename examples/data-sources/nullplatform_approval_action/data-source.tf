data "nullplatform_notification_channel" "slack" {
  nrn = nullplatform_notification_channel.slack.nrn
}

data "nullplatform_notification_channel" "webhook" {
  nrn = nullplatform_notification_channel.webhook.nrn
}

data "nullplatform_notification_channel" "github" {
  nrn = nullplatform_notification_channel.github.nrn
}

output "slack_channel_details" {
  value = data.nullplatform_notification_channel.slack
}

output "webhook_channel_details" {
  value = data.nullplatform_notification_channel.webhook
}

output "github_channel_details" {
  value = data.nullplatform_notification_channel.github
}
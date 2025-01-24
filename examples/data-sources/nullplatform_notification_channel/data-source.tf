data "nullplatform_notification_channel" "slack" {
  nrn = "organization=1:account=2:namespace=3:application=123"
}

data "nullplatform_notification_channel" "webhook" {
  nrn = "organization=1:account=2:namespace=3:application=123"
}

data "nullplatform_notification_channel" "github" {
  nrn = "organization=1:account=2:namespace=3:application=123"
}
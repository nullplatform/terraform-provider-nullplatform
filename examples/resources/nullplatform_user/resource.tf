terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_user" "example" {
  email      = "jane.doe@example.com"
  first_name = "Jane"
  last_name  = "Doe"

  # Optional avatar image URL shown in the nullplatform UI
  avatar = "https://example.com/avatars/jane-doe.png"

  # When false, reuse an existing user with this email instead of failing
  strict = false
}

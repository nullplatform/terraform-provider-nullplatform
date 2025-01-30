resource "nullplatform_user" "simple" {
  email      = "user@example.com"
  first_name = "John"
  last_name  = "Doe"
}

resource "nullplatform_user" "with_avatar" {
  email      = "jane@example.com"
  first_name = "Jane"
  last_name  = "Smith"
  avatar     = "https://example.com/avatar.jpg"
}

resource "nullplatform_user" "developer1" {
  email      = "dev1@example.com"
  first_name = "Alice"
  last_name  = "Developer"
}

resource "nullplatform_user" "developer2" {
  email      = "dev2@example.com"
  first_name = "Bob"
  last_name  = "Engineer"
}
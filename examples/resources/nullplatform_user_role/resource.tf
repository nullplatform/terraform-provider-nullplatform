resource "nullplatform_user" "admin" {
  email      = "admin@example.com"
  first_name = "Jane"
  last_name  = "Admin"
}

# Grant organization admin role
resource "nullplatform_user_role" "org_admin" {
  user_id   = nullplatform_user.admin.id
  role_slug = "organization:admin"
  nrn       = "organization=1234567890"
}

# Grant account developer role
resource "nullplatform_user_role" "account_dev" {
  user_id   = nullplatform_user.admin.id
  role_slug = "account:developer"
  nrn       = "organization=1234567890:account=9876543210"
}

# Grant namespace operations role
resource "nullplatform_user_role" "namespace_ops" {
  user_id   = nullplatform_user.admin.id
  role_slug = "namespace:ops"
  nrn       = "organization=1234567890:account=9876543210:namespace=5555555555"
}
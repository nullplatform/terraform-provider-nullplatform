# Find a package published for every organization, then install it in one
# account. New services there are born with the revision installed here.
data "nullplatform_package" "lambda" {
  visible_at = "organization=123:account=456"
  slug       = "scopes-lambda"
}

resource "nullplatform_package_installation" "lambda" {
  nrn                 = "organization=123:account=456"
  package_id          = data.nullplatform_package.lambda.id
  default_revision_id = data.nullplatform_package.lambda.revision_id

  # Optional: where inside the organization the specification is usable.
  # Defaults to [nrn].
  visible_to = ["organization=123:account=456:namespace=789"]
}

# Providers the package can use but the organization has not configured yet.
output "missing_optional_requirements" {
  value = nullplatform_package_installation.lambda.missing_optional_requirements
}

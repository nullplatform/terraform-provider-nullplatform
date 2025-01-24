data "nullplatform_approval_policy" "example" {
  nrn = nullplatform_approval_policy.example.nrn
}

data "nullplatform_approval_policy" "example_with_account_name" {
  nrn = nullplatform_approval_policy.example_with_account_name.nrn
}

output "example_policy_conditions" {
  value = data.nullplatform_approval_policy.example.conditions
}

output "example_account_policy_conditions" {
  value = data.nullplatform_approval_policy.example_with_account_name.conditions
}
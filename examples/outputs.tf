output "scope" {
  value = nullplatform_scope.test
}

#output "scope_var" {
#  value = nullplatform_scope.test1.id
#}
#
#data "nullplatform_scope" "first" {
#  id = nullplatform_scope.test1.id
#}
#
#output "first_order" {
#  value = data.nullplatform_scope.first
#}

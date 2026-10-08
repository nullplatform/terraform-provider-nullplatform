# A checklist specification can be imported by the id of any of its versions, or
# by "<nrn>/<name>". Either way it lands on its current version.
terraform import nullplatform_checklist_specification.deployment_review spec_0123456789abcdef
terraform import nullplatform_checklist_specification.deployment_review "organization=1:account=2:namespace=3:application=4/deployment-review"

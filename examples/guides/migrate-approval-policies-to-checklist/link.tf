resource "nullplatform_approval_action_checklist_specification_association" "deployment_create" {
  approval_action_id         = nullplatform_approval_action.deployment_create.id
  checklist_specification_id = nullplatform_checklist_specification.deployment_review.current_version_id
}

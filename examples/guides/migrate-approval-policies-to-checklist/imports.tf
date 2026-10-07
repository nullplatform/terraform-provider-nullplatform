# The ids come from GET /approval/action/<id>: its id, and the
# checklist_specification_id the migration linked.
import {
  to = nullplatform_checklist_specification.deployment_review
  id = "spec_0123456789abcdef"
}

import {
  to = nullplatform_approval_action_checklist_specification_association.deployment_create
  id = "123"
}

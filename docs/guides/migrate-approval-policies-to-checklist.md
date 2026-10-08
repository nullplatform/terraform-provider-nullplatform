---
page_title: "Migrate approval policies to checklist specifications"
subcategory: ""
description: |-
  Move an approval action from policies to a checklist specification, from Terraform, and back.
---

# Migrate approval policies to checklist specifications

An approval action decides its requests either with **policies** or with a **checklist specification**, never both. This guide
moves an action managed with Terraform from policies to a specification, and back. Three resources take part:

- `nullplatform_checklist_specification`: the checklist. Every change of its `definition` or `description` creates a new
  version, and the same apply re-links every action whose link references `current_version_id`; requests already running keep
  their version.
- `nullplatform_approval_action_checklist_specification_association`:
  links an action to a specification version. An action takes a link only once it has no live policy associations.
- `nullplatform_approval_action`: unchanged in what you already declare. It adds
  `on_checklist_fail`, and reads the linked version in `checklist_specification_id`.

Upgrading the provider changes nothing by itself: an existing configuration plans no changes, except one whose `entity` or
`action` differs from the API's: that diff never applied, and it now plans to replace the action. Set them back to what the API
holds to keep the action.

## Before you start

The API key that runs Terraform needs permission to manage checklist specifications and to link approval actions to them, on the
NRN of the specification or of the action, besides the permissions it already uses for approval actions and policies: without
them, the apply fails with a 403. Importing a specification by `<nrn>/<name>` also needs permission to read the specifications of
that NRN.

## Writing the specification

Write the checklist that does what the policies did, as a migration with the API does. Its format is documented in
[Specs and actions](https://docs.nullplatform.com/docs/approvals/checklist-specs).

- **Each policy condition becomes a `condition` item** with its `query`: the condition's key and expression, as the policy had
  them. A policy with `{ "build.metadata.coverage.code.coverage" = { "$gte" = 80 } }` gives an item with that `query`, never a
  `manual` one.
- **The reviewer.** If the action's `on_policy_success` is `manual`, which is the default, or the action had no policies, end
  `items` with `{ id = "human_sign_off", type = "manual", behavior = "gate", title = "A reviewer signs off" }`. Without that
  item, requests that pass the conditions are approved with no reviewer.
- **`execution_trigger`.** The `scope:create` and `scope:write` actions use `"any_approval"`, whatever `on_policy_success` is.
  Another action with `on_policy_success = "approve"` and at least one policy approved its requests automatically: use
  `"automatic_approval"`, with no `manual` or `external` items. With `automatic_approval`, a request whose items pass is
  approved and its execution starts with no one triggering it; a held request, such as a deployment in a group, still waits.
  Any other action uses `"explicit"`, which is also what an absent `execution_trigger` means.
- **`on_checklist_fail`** on the action does the job `on_policy_fail` did. When it is not set, linking a specification sets it to
  `deny` if `on_policy_fail` is `deny`, and to `pending` otherwise. Set `manual` to send failed checklists to a reviewer.

An action with `on_policy_success = "manual"` (the default):

```terraform
# The action had on_policy_success = "manual", the default: a reviewer signed
# off after the conditions passed. The last item keeps that sign-off.
resource "nullplatform_checklist_specification" "deployment_review" {
  nrn         = "organization=1:account=2:namespace=3:application=4"
  name        = "deployment-review"
  description = "What deployment_create checked with policies"

  definition = jsonencode({
    execution_trigger = "explicit"
    items = [
      {
        id       = "code_coverage"
        type     = "condition"
        behavior = "gate"
        title    = "Code coverage of at least 80%"
        query    = { "build.metadata.coverage.code.coverage" = { "$gte" = 80 } }
      },
      {
        id       = "human_sign_off"
        type     = "manual"
        behavior = "gate"
        title    = "A reviewer signs off"
      }
    ]
  })
}
```

An action with `on_policy_success = "approve"`:

```terraform
# The action had on_policy_success = "approve": requests that pass the
# conditions are approved with no reviewer, as they were with policies.
resource "nullplatform_checklist_specification" "deployment_review" {
  nrn         = "organization=1:account=2:namespace=3:application=4"
  name        = "deployment-review"
  description = "What deployment_create checked with policies"

  definition = jsonencode({
    execution_trigger = "automatic_approval"
    items = [
      {
        id       = "code_coverage"
        type     = "condition"
        behavior = "gate"
        title    = "Code coverage of at least 80%"
        query    = { "build.metadata.coverage.code.coverage" = { "$gte" = 80 } }
      }
    ]
  })
}
```

The link always references `current_version_id`, never the specification's `id` (`<nrn>/<name>`, which the link refuses):

```terraform
resource "nullplatform_approval_action_checklist_specification_association" "deployment_create" {
  approval_action_id         = nullplatform_approval_action.deployment_create.id
  checklist_specification_id = nullplatform_checklist_specification.deployment_review.current_version_id
}
```

## An action with policy association resources

An action that ignores `policies` and manages them with association resources: `lifecycle { ignore_changes = [policies] }` on the
action and one `nullplatform_approval_action_policy_association` per policy. Keep the action block as it is, **delete** the
association blocks, and add the specification and the link:

```terraform
# Unchanged: the action block stays as it was.
resource "nullplatform_approval_action" "deployment_create" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail    = "manual"

  lifecycle {
    ignore_changes = [policies]
  }
}
```

One apply does it: the link waits, up to its `timeouts.create` (one minute by default), for the associations to be deleted. Until
it lands the action has no rules, and a request created then goes to human review; it is never approved on its own. The next plan
shows no changes.

The `nullplatform_approval_policy` resources stay: the provider never removes policies, nor associations it does not manage.

## An action with inline `policies`

Remove `policies` from the action and add the specification and the link:

```terraform
# policies is removed from the action; the policy resources can stay.
resource "nullplatform_approval_action" "deployment_create" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail    = "manual"
}
```

```
Plan: 2 to add, 1 to change, 0 to destroy.
```

## An action already migrated by the API

An action migrated with the API is already linked, and its policy associations are archived. Until its configuration is updated,
every plan warns once per association resource:

```
Warning: Policy association has no effect: the approval action uses a checklist specification
```

An action that lists its policies inline gets no warning.

Declare the specification the migration linked (`checklist_specification_id` in the action's API response) and the link, import
both, and delete the association blocks. If the action lists its policies inline, replace `policies` with
`lifecycle { ignore_changes = [policies] }`: without it, every plan after an unlink proposes removing policies that no longer take
part.

```terraform
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
```

or, without `import` blocks:

```shell
terraform import nullplatform_checklist_specification.deployment_review spec_0123456789abcdef
terraform import nullplatform_approval_action_checklist_specification_association.deployment_create 123
```

The plan imports two and destroys the associations; deleting an archived association changes nothing in the API. If the
`definition` you wrote differs from the one the migration generated, the plan also creates a new version and re-links the action.

On a migrated action, `policies` and association resources have no effect, and adding a policy fails with
`Approval action already has a checklist specification; cannot also associate policies`.

## Unlinking, or going back to policies

Delete the link block to unlink. The action is left with neither a specification nor live policies, so its requests go to human
review until it gets one or the other. After an unlink alone, the state of an action the API migrated still lists the archived
policies: there, `No changes` does not mean a policy is live.

Go back with the form the action had: moving it from association resources to inline `policies` in the same apply removes the
policy for that apply, and the next plan adds it again. An action the API migrated is the exception, below.

To go back to policies with association resources, delete the link block and add the associations in the same apply. The
action needs `lifecycle { ignore_changes = [policies] }`: without it, the next plan proposes removing the policies the associations
added, and applying that plan removes them.

```terraform
# The link block is deleted and the association is added, in one apply. The
# action keeps ignore_changes = [policies]: without it, the next plan would
# remove the policy the association adds.
resource "nullplatform_approval_action" "deployment_create" {
  nrn    = "organization=1:account=2:namespace=3:application=4"
  entity = "deployment"
  action = "deployment:create"

  dimensions = {
    environment = "production"
  }

  on_policy_success = "approve"
  on_policy_fail    = "manual"

  lifecycle {
    ignore_changes = [policies]
  }
}

resource "nullplatform_approval_action_policy_association" "coverage" {
  approval_action_id = nullplatform_approval_action.deployment_create.id
  approval_policy_id = nullplatform_approval_policy.coverage.id
}
```

An action that had inline `policies` goes back by listing them in `policies` again, with no association resources.

An action the API migrated goes back only with association resources and `ignore_changes = [policies]`, also when its
configuration listed the policies inline: listing them in `policies` again re-associates nothing and plans `No changes`, while its
requests go to human review.

## Limitations

- **Actions linked outside Terraform** stay on their version when the specification changes. Declare them in Terraform, or move
  them with the API.
- **A deleted name cannot be created again** at the same NRN (`destroy` then `apply`, `taint`, `-replace`): the apply fails with
  `A checklist specification with the same nrn/name/version already exists`. Use another `name`.
- **`-parallelism=1`**, migrating an action with association resources: the link can hold the only worker until its
  `timeouts.create` runs out. Apply twice: first delete the associations, then add the specification and the link.
- **An action already linked to another version** is not taken over: import the link. One already linked to the declared version
  is adopted.
- **One link per action.**

## Troubleshooting

- **`approval action <id> still has policies <ids> after 1m0s`**: the action still had a live policy association, usually one this
  configuration does not manage: import or delete it, and apply again. The ids include policies a past migration archived, which do
  not block the link.
- **`error associating approval policy with action, got status code: 409`**: the action is still linked to a specification a minute
  after the association started. Delete the link resource and apply again.

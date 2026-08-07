---
page_title: "extract_scope_id function - nullplatform"
subcategory: ""
description: |-
  Extract the scope id from an NRN
---

# Function: extract_scope_id

Given an NRN (Nullplatform Resource Name), returns the id of its `scope` component. Returns an error if the NRN does not contain a `scope` component.

~> Provider-defined functions require Terraform 1.8 or later.

## Example Usage

```terraform
locals {
  # var.scope_nrn = "organization=1:account=2:namespace=3:application=4:scope=5"
  scope_id = provider::nullplatform::extract_scope_id(var.scope_nrn)
}
```

## Signature

```text
extract_scope_id(nrn string) string
```

## Arguments

1. `nrn` (String) The NRN to extract the id from (e.g. `organization=1:account=2:namespace=3`).

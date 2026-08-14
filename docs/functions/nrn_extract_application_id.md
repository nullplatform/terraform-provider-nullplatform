---
page_title: "nrn_extract_application_id function - nullplatform"
subcategory: ""
description: |-
  Extract the application id from an NRN
---

# Function: nrn_extract_application_id

Given an NRN (Nullplatform Resource Name), returns the id of its `application` component. Returns an error if the NRN does not contain an `application` component.

~> Provider-defined functions require Terraform 1.8 or later.

## Example Usage

```terraform
locals {
  # var.application_nrn = "organization=1:account=2:namespace=3:application=4"
  application_id = provider::nullplatform::nrn_extract_application_id(var.application_nrn)
}

data "nullplatform_application" "this" {
  id = local.application_id
}
```

## Signature

```text
nrn_extract_application_id(nrn string) string
```

## Arguments

1. `nrn` (String) The NRN to extract the id from (e.g. `organization=1:account=2:namespace=3`).

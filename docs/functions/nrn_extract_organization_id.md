---
page_title: "nrn_extract_organization_id function - nullplatform"
subcategory: ""
description: |-
  Extract the organization id from an NRN
---

# Function: nrn_extract_organization_id

Given an NRN (Nullplatform Resource Name), returns the id of its `organization` component. Returns an error if the NRN does not contain an `organization` component.

~> Provider-defined functions require Terraform 1.8 or later.

## Example Usage

```terraform
locals {
  # var.nrn = "organization=1255165411:account=95118862"
  organization_id = provider::nullplatform::nrn_extract_organization_id(var.nrn)
}
```

## Signature

```text
nrn_extract_organization_id(nrn string) string
```

## Arguments

1. `nrn` (String) The NRN to extract the id from (e.g. `organization=1:account=2:namespace=3`).

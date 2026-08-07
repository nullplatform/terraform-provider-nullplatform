---
page_title: "extract_namespace_id function - nullplatform"
subcategory: ""
description: |-
  Extract the namespace id from an NRN
---

# Function: extract_namespace_id

Given an NRN (Nullplatform Resource Name), returns the id of its `namespace` component. Returns an error if the NRN does not contain a `namespace` component.

~> Provider-defined functions require Terraform 1.8 or later.

## Example Usage

```terraform
locals {
  # var.namespace_nrn = "organization=1255165411:account=95118862:namespace=1991376853"
  namespace_id = provider::nullplatform::extract_namespace_id(var.namespace_nrn)
}
```

## Signature

```text
extract_namespace_id(nrn string) string
```

## Arguments

1. `nrn` (String) The NRN to extract the id from (e.g. `organization=1:account=2:namespace=3`).

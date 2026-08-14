---
page_title: "nrn_extract_account_id function - nullplatform"
subcategory: ""
description: |-
  Extract the account id from an NRN
---

# Function: nrn_extract_account_id

Given an NRN (Nullplatform Resource Name), returns the id of its `account` component. Returns an error if the NRN does not contain an `account` component.

~> Provider-defined functions require Terraform 1.8 or later.

## Example Usage

```terraform
locals {
  # var.namespace_nrn = "organization=1255165411:account=95118862:namespace=1991376853"
  account_id = provider::nullplatform::nrn_extract_account_id(var.namespace_nrn)
}
```

## Signature

```text
nrn_extract_account_id(nrn string) string
```

## Arguments

1. `nrn` (String) The NRN to extract the id from (e.g. `organization=1:account=2:namespace=3`).

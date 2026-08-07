---
page_title: "nrn_extract_id function - nullplatform"
subcategory: ""
description: |-
  Extract an entity id from an NRN
---

# Function: nrn_extract_id

Given an entity type and an NRN (Nullplatform Resource Name), returns the id of the NRN component for that entity type. Returns an error if the NRN does not contain the requested entity type.

Valid entity types include `organization`, `account`, `namespace`, `application`, and `scope`.

Use this function when the entity type comes from a variable; when it is fixed, the dedicated variants (`nrn_extract_account_id`, `nrn_extract_namespace_id`, etc.) are more direct.

~> Provider-defined functions require Terraform 1.8 or later.

## Example Usage

```terraform
locals {
  # var.namespace_nrn = "organization=1255165411:account=95118862:namespace=1991376853"
  account_id   = provider::nullplatform::nrn_extract_id("account", var.namespace_nrn)
  namespace_id = provider::nullplatform::nrn_extract_id("namespace", var.namespace_nrn)
}
```

## Signature

```text
nrn_extract_id(entity_type string, nrn string) string
```

## Arguments

1. `entity_type` (String) The entity type whose id should be extracted (e.g. `account`, `namespace`).
2. `nrn` (String) The NRN to extract the id from (e.g. `organization=1:account=2:namespace=3`).

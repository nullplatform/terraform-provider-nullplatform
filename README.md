# terraform-provider-nullplatform

The Provider allows Terraform to manage Null Platform resources.


## Development Environment Setup

### Requirements

* Terraform 0.12.26+ (to run acceptance tests)
* Go 1.21+ (to build the provider plugin)

### Building the Provider

```shell
make
```

This will build the provider and move the binary to the Terraform plugins directory.

### Debugging the Provider

* Export GOBIN
```bash
export GOBIN=$HOME/go/bin
``` 
* Create this `~/.terraformrc` file
```bash
echo Test if either $HOME or ~ works, otherwise full path works
provider_installation {

  dev_overrides {
      "registry.terraform.io/nullplatform/nullplatform" = "$HOME/go/bin"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```
* Create terraform code to use the provider (like the ones in examples)
* Without doing Terraform init, perform a plan or apply whereas needed


### Using the Provider

With Terraform v0.14 and later, [development overrides for provider developers](https://www.terraform.io/cli/config/config-file#development-overrides-for-provider-developers) can be leveraged in order to use the provider built from source.

To do this, populate a Terraform CLI configuration file `~/.terraformrc` with at least the following options:

```
provider_installation {
  dev_overrides {
    "nullplatform/nullplatform" = "[REPLACE PATH WITH THE OUTPUT OF THE MAKE COMMAND]"
  }
  direct {}
}
```

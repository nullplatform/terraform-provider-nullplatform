<h2 align="center">
    <a href="https://nullplatform.com" target="blank_">
        <img height="100" alt="Nullplatform" src="https://nullplatform.com/favicon/android-chrome-192x192.png" />
    </a>
    <br>
    <br>
    Terraform provider nullplatform
    <br>
</h2>

This provider allows Terraform to manage nullplatform resources.

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
### Documenting the Provider
If it is the first time you are documenting a resource, you need to run the following command to install the `tfplugindocs` tool:
```bash 
export GOBIN=$PWD/bin
export PATH=$GOBIN:$PATH
go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs
which tfplugindocs
```

After that, you can run the following command to autogenerate the documentation for the provider:
```bash
make update-docs
```
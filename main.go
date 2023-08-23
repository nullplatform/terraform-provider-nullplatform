package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: nullplatform.Provider,
	})
}

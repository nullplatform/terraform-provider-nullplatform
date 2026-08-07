package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

// Run "go generate" to format example terraform files and generate the docs for the registry/website

// If you do not have terraform installed, you can remove the formatting command, but its suggested to
// ensure the documentation is formatted properly.
//go:generate terraform fmt -recursive ./examples/

// Run the docs generation tool, check its repository for more information on how it works and how docs
// can be customized.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate -provider-name nullplatform

// The provider is served as a mux of two servers: the terraform-plugin-sdk/v2
// provider (resources and data sources) and a terraform-plugin-framework
// provider that only contributes provider-defined functions, which SDKv2
// cannot express.
func main() {
	ctx := context.Background()

	providers := []func() tfprotov5.ProviderServer{
		nullplatform.Provider().GRPCProvider,
		providerserver.NewProtocol5(nullplatform.NewFrameworkProvider()),
	}

	muxServer, err := tf5muxserver.NewMuxServer(ctx, providers...)
	if err != nil {
		log.Fatal(err)
	}

	if err := tf5server.Serve(
		"registry.terraform.io/nullplatform/nullplatform",
		muxServer.ProviderServer,
	); err != nil {
		log.Fatal(err)
	}
}

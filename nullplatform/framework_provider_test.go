package nullplatform

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
)

// The mux server requires the SDKv2 provider and the framework provider to
// expose identical provider configuration schemas. Any drift between
// provider.go and framework_provider.go (a new attribute, a reworded
// description, a changed deprecation message) breaks the muxed server at
// runtime, so this test exercises the same combination main.go serves.
func TestMuxServer_ProviderSchemasAreCompatible(t *testing.T) {
	ctx := context.Background()

	muxServer, err := tf5muxserver.NewMuxServer(ctx,
		Provider().GRPCProvider,
		providerserver.NewProtocol5(NewFrameworkProvider()),
	)
	if err != nil {
		t.Fatalf("failed to create mux server: %v", err)
	}

	resp, err := muxServer.ProviderServer().GetProviderSchema(ctx, &tfprotov5.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema failed: %v", err)
	}

	for _, diagnostic := range resp.Diagnostics {
		if diagnostic.Severity == tfprotov5.DiagnosticSeverityError {
			t.Errorf("provider schemas are not mux-compatible: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
}

func TestMuxServer_ServesExtractFunctions(t *testing.T) {
	ctx := context.Background()

	muxServer, err := tf5muxserver.NewMuxServer(ctx,
		Provider().GRPCProvider,
		providerserver.NewProtocol5(NewFrameworkProvider()),
	)
	if err != nil {
		t.Fatalf("failed to create mux server: %v", err)
	}

	resp, err := muxServer.ProviderServer().GetProviderSchema(ctx, &tfprotov5.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema failed: %v", err)
	}

	wantFunctions := []string{
		"nrn_extract_id",
		"nrn_extract_organization_id",
		"nrn_extract_account_id",
		"nrn_extract_namespace_id",
		"nrn_extract_application_id",
		"nrn_extract_scope_id",
	}

	for _, name := range wantFunctions {
		if _, ok := resp.Functions[name]; !ok {
			t.Errorf("function %q is not served by the muxed provider", name)
		}
	}
}

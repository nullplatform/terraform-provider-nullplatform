package nullplatform

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const testNRN = "organization=1255165411:account=95118862:namespace=1991376853:application=1152869225:scope=272433493"

func TestExtractIDFromNRN(t *testing.T) {
	tests := []struct {
		name       string
		nrn        string
		entityType string
		want       string
		wantErr    string
	}{
		{name: "organization", nrn: testNRN, entityType: "organization", want: "1255165411"},
		{name: "account", nrn: testNRN, entityType: "account", want: "95118862"},
		{name: "namespace", nrn: testNRN, entityType: "namespace", want: "1991376853"},
		{name: "application", nrn: testNRN, entityType: "application", want: "1152869225"},
		{name: "scope", nrn: testNRN, entityType: "scope", want: "272433493"},
		{name: "partial nrn", nrn: "organization=1:account=2", entityType: "account", want: "2"},
		{name: "entity type is case-insensitive", nrn: testNRN, entityType: "Account", want: "95118862"},
		{name: "surrounding whitespace is tolerated", nrn: "  " + testNRN + "  ", entityType: " account ", want: "95118862"},
		{name: "missing entity type", nrn: "organization=1:account=2", entityType: "application", wantErr: "does not contain an id"},
		{name: "empty nrn", nrn: "", entityType: "account", wantErr: "NRN must not be empty"},
		{name: "empty entity type", nrn: testNRN, entityType: "", wantErr: "entity type must not be empty"},
		{name: "malformed segment", nrn: "organization=1:account", entityType: "account", wantErr: "not in \"<entity>=<id>\" format"},
		{name: "empty segment value", nrn: "organization=1:account=", entityType: "account", wantErr: "not in \"<entity>=<id>\" format"},
		{name: "not an nrn at all", nrn: "hello world", entityType: "account", wantErr: "not in \"<entity>=<id>\" format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractIDFromNRN(tt.nrn, tt.entityType)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got id %q", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func runFunction(t *testing.T, f function.Function, args []attr.Value) *function.RunResponse {
	t.Helper()

	resp := &function.RunResponse{
		Result: function.NewResultData(types.StringUnknown()),
	}
	f.Run(context.Background(), function.RunRequest{
		Arguments: function.NewArgumentsData(args),
	}, resp)

	return resp
}

func TestExtractIDFunction_Run(t *testing.T) {
	resp := runFunction(t, NewExtractIDFunction(), []attr.Value{
		types.StringValue("account"),
		types.StringValue(testNRN),
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if got, want := resp.Result.Value(), types.StringValue("95118862"); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractIDFunction_RunErrorsWhenEntityTypeMissing(t *testing.T) {
	resp := runFunction(t, NewExtractIDFunction(), []attr.Value{
		types.StringValue("scope"),
		types.StringValue("organization=1:account=2"),
	})

	if resp.Error == nil {
		t.Fatal("expected an error for an NRN without the requested entity type, got none")
	}
	if !strings.Contains(resp.Error.Error(), "does not contain an id") {
		t.Errorf("unexpected error message: %v", resp.Error)
	}
}

func TestExtractEntityIDFunctions_Run(t *testing.T) {
	tests := []struct {
		newFunc  func() function.Function
		wantName string
		want     string
	}{
		{NewExtractOrganizationIDFunction, "extract_organization_id", "1255165411"},
		{NewExtractAccountIDFunction, "extract_account_id", "95118862"},
		{NewExtractNamespaceIDFunction, "extract_namespace_id", "1991376853"},
		{NewExtractApplicationIDFunction, "extract_application_id", "1152869225"},
		{NewExtractScopeIDFunction, "extract_scope_id", "272433493"},
	}

	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			f := tt.newFunc()

			metaResp := &function.MetadataResponse{}
			f.Metadata(context.Background(), function.MetadataRequest{}, metaResp)
			if metaResp.Name != tt.wantName {
				t.Errorf("function name = %q, want %q", metaResp.Name, tt.wantName)
			}

			resp := runFunction(t, f, []attr.Value{types.StringValue(testNRN)})
			if resp.Error != nil {
				t.Fatalf("unexpected error: %v", resp.Error)
			}
			if got, want := resp.Result.Value(), types.StringValue(tt.want); !got.Equal(want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestExtractEntityIDFunction_RunErrorsWhenComponentMissing(t *testing.T) {
	resp := runFunction(t, NewExtractScopeIDFunction(), []attr.Value{
		types.StringValue("organization=1:account=2"),
	})

	if resp.Error == nil {
		t.Fatal("expected an error for an NRN without a scope component, got none")
	}
}

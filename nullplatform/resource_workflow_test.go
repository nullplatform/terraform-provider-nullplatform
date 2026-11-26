package nullplatform

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
)

func TestValidateWorkflowPath(t *testing.T) {
	for _, ok := range []string{"/a/b", "/", "/governance/releases"} {
		if _, errs := validateWorkflowPath(ok, "path"); len(errs) != 0 {
			t.Errorf("%q refused: %v", ok, errs)
		}
	}
	for _, bad := range []string{"", "a", "/a/", "//a", "/a//b", "a/b"} {
		if _, errs := validateWorkflowPath(bad, "path"); len(errs) != 1 {
			t.Errorf("%q accepted, want one error", bad)
		}
	}
	_, errs := validateWorkflowPath("/a/", "path")
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), `"/a"`) {
		t.Errorf("error = %v, want it to name the normalized form", errs)
	}
	_, errs = validateWorkflowPath("", "path")
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "must not be empty") {
		t.Errorf("error = %v, want the empty-path message", errs)
	}
}

func TestValidateWorkflowDefinition(t *testing.T) {
	path := cty.GetAttrPath("definition")
	for _, ok := range []string{`{"steps":{}}`, `{"steps":{"a":{"pluginType":"webhook"}},"connections":[]}`} {
		if diags := validateWorkflowDefinition(ok, path); len(diags) != 0 {
			t.Errorf("%s refused: %v", ok, diags)
		}
	}
	mustFail := map[string]string{
		"not JSON":          `{nope`,
		"not an object":     `[1]`,
		"null":              `null`,
		"no steps":          `{"connections":[]}`,
		"steps not object":  `{"steps":[]}`,
		"steps null":        `{"steps":null}`,
		"steps is a string": `{"steps":"x"}`,
	}
	for name, def := range mustFail {
		diags := validateWorkflowDefinition(def, path)
		if !diags.HasError() {
			t.Errorf("%s: accepted %s", name, def)
		}
	}
	for _, key := range append(append([]string{}, workflowRecordKeys...), workflowDerivedKeys...) {
		def := `{"steps":{},"` + key + `":"x"}`
		diags := validateWorkflowDefinition(def, path)
		if len(diags) != 1 || !diags.HasError() || !strings.Contains(diags[0].Summary, `"`+key+`"`) {
			t.Errorf("reserved key %q: diags = %v", key, diags)
		}
	}
	// Every reserved key is reported, not only the first.
	diags := validateWorkflowDefinition(`{"steps":{},"name":"x","id":"y"}`, path)
	if len(diags) != 2 {
		t.Errorf("two reserved keys gave %d diagnostics, want 2", len(diags))
	}
}

func TestWorkflowDefinitionsEquivalent(t *testing.T) {
	const written = `{"steps":{"a":{"id":"a","pluginType":"webhook","config":{"path":"/x"}}},"connections":[]}`
	tests := []struct {
		name            string
		written, stored string
		want            bool
	}{
		{"identical", written, written, true},
		{"key order and whitespace",
			written,
			"{\n \"connections\": [],\n \"steps\": {\"a\": {\"config\": {\"path\": \"/x\"}, \"pluginType\": \"webhook\", \"id\": \"a\"}}\n}", true},
		{"record and derived keys in the stored one", written,
			`{"name":"payments-gate","key":"k","description":"d","path":"/p","nrn":"organization=1","id":"wf_1",` +
				`"triggers":[{"id":"a"}],"pinnedDescriptors":{},"steps":{"a":{"id":"a","pluginType":"webhook","config":{"path":"/x"}}},"connections":[]}`, true},
		{"a step type filled by the API", written,
			`{"steps":{"a":{"id":"a","type":"trigger","pluginType":"webhook","config":{"path":"/x"}}},"connections":[]}`, true},
		{"a step type the user wrote and the API kept",
			`{"steps":{"a":{"type":"trigger","pluginType":"webhook"}}}`,
			`{"steps":{"a":{"type":"trigger","pluginType":"webhook"}}}`, true},
		{"a step type the user wrote and the API changed",
			`{"steps":{"a":{"type":"trigger","pluginType":"webhook"}}}`,
			`{"steps":{"a":{"type":"action","pluginType":"webhook"}}}`, false},
		{"a step type the user wrote and the API dropped",
			`{"steps":{"a":{"type":"trigger","pluginType":"webhook"}}}`,
			`{"steps":{"a":{"pluginType":"webhook"}}}`, false},
		{"a changed config", written,
			`{"steps":{"a":{"id":"a","pluginType":"webhook","config":{"path":"/y"}}},"connections":[]}`, false},
		{"an extra step in the stored one", written,
			`{"steps":{"a":{"id":"a","pluginType":"webhook","config":{"path":"/x"}},"b":{}},"connections":[]}`, false},
		{"an extra section in the stored one", written,
			`{"steps":{"a":{"id":"a","pluginType":"webhook","config":{"path":"/x"}}},"connections":[],"inputs":{}}`, false},
		{"a written untyped step vs a stored non-object step",
			`{"steps":{"a":{"pluginType":"x"}}}`, `{"steps":{"a":"oops"}}`, false},
		{"written is not JSON", `{nope`, written, false},
		{"stored is not JSON", written, `{nope`, false},
		{"both are not JSON", `x`, `x`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workflowDefinitionsEquivalent(tt.written, tt.stored); got != tt.want {
				t.Errorf("equivalent = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkflowStateDefinition(t *testing.T) {
	t.Run("an equivalent stored definition keeps the one written", func(t *testing.T) {
		written := "{ \"steps\": { \"a\": {\"pluginType\":\"webhook\"} } }"
		stored := `{"id":"wf_1","name":"n","steps":{"a":{"pluginType":"webhook","type":"trigger"}},"triggers":[]}`
		got, err := workflowStateDefinition(written, []byte(stored))
		if err != nil || got != written {
			t.Errorf("got %q, %v; want the written definition untouched", got, err)
		}
	})
	t.Run("a different stored definition is stripped, compact, sorted, unescaped", func(t *testing.T) {
		stored := `{"zeta":1,"name":"n","key":"k","description":"d","path":"/p","nrn":"organization=1","id":"wf_1",` +
			`"triggers":[],"pinnedDescriptors":{},"alpha":2,` +
			`"steps":{"a":{"type":"trigger","config":{"url":"https://example.test/x?a=1&b=<2>"}}}}`
		got, err := workflowStateDefinition(`{"steps":{}}`, []byte(stored))
		if err != nil {
			t.Fatal(err)
		}
		want := `{"alpha":2,"steps":{"a":{"config":{"url":"https://example.test/x?a=1&b=<2>"},"type":"trigger"}},"zeta":1}`
		if got != want {
			t.Errorf("got  %s\nwant %s", got, want)
		}
	})
	t.Run("a non-object answer is an error", func(t *testing.T) {
		for _, stored := range []string{`[1]`, `"text"`, `12`} {
			if got, err := workflowStateDefinition(`{"steps":{}}`, []byte(stored)); err == nil {
				t.Errorf("stored %s gave %q, want an error", stored, got)
			}
		}
	})
}

func TestSortedKeys(t *testing.T) {
	if got, want := sortedKeys(map[string]int{"b": 1, "c": 2, "a": 3}), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("sortedKeys = %v, want %v", got, want)
	}
	if got := sortedKeys(map[string]bool(nil)); len(got) != 0 {
		t.Errorf("sortedKeys(nil) = %v, want empty", got)
	}
}

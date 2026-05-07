package nullplatform

import (
	"strings"
	"testing"
)

func TestFindActionSpecByType_HappyPath(t *testing.T) {
	specs := []*ActionSpecification{
		{Id: "1", Type: "create"},
		{Id: "2", Type: "delete"},
	}
	got, err := findActionSpecByType(specs, "create")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Id != "1" {
		t.Errorf("got id %q, want %q", got.Id, "1")
	}
}

func TestFindActionSpecByType_FirstMatchWins(t *testing.T) {
	specs := []*ActionSpecification{
		{Id: "1", Type: "create"},
		{Id: "2", Type: "create"},
	}
	got, _ := findActionSpecByType(specs, "create")
	if got.Id != "1" {
		t.Errorf("expected first match (id=1), got id=%q", got.Id)
	}
}

func TestFindActionSpecByType_NotFound(t *testing.T) {
	specs := []*ActionSpecification{
		{Id: "1", Type: "create"},
	}
	_, err := findActionSpecByType(specs, "delete")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "delete") {
		t.Errorf("error %q should mention the missing type", err.Error())
	}
}

func TestFindActionSpecByType_EmptyInput(t *testing.T) {
	_, err := findActionSpecByType(nil, "create")
	if err == nil {
		t.Fatal("expected error on nil input, got nil")
	}
}

func TestProjectAttributesToParameters_HappyPath(t *testing.T) {
	attrs := map[string]interface{}{"endpoint": "redis.local", "port": 6379, "extra": "ignored"}
	schema := map[string]interface{}{
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"endpoint": map[string]interface{}{"type": "string"},
				"port":     map[string]interface{}{"type": "number"},
			},
		},
		"values": map[string]interface{}{},
	}
	got, _ := projectAttributesToParameters(attrs, schema)
	if len(got) != 2 {
		t.Fatalf("expected 2 keys, got %d (%v)", len(got), got)
	}
	if got["endpoint"] != "redis.local" || got["port"] != 6379 {
		t.Errorf("unexpected values: %v", got)
	}
	if _, exists := got["extra"]; exists {
		t.Errorf("expected 'extra' to be filtered out")
	}
}

func TestProjectAttributesToParameters_NoSchemaKey(t *testing.T) {
	attrs := map[string]interface{}{"endpoint": "redis.local"}
	schema := map[string]interface{}{"values": map[string]interface{}{}}
	got, _ := projectAttributesToParameters(attrs, schema)
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestProjectAttributesToParameters_NoPropertiesKey(t *testing.T) {
	attrs := map[string]interface{}{"endpoint": "redis.local"}
	schema := map[string]interface{}{"schema": map[string]interface{}{"type": "object"}}
	got, _ := projectAttributesToParameters(attrs, schema)
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestProjectAttributesToParameters_PropertiesNotAnObject(t *testing.T) {
	attrs := map[string]interface{}{"endpoint": "redis.local"}
	schema := map[string]interface{}{"schema": map[string]interface{}{"properties": "not-an-object"}}
	got, _ := projectAttributesToParameters(attrs, schema)
	if len(got) != 0 {
		t.Errorf("expected empty map on malformed schema, got %v", got)
	}
}

func TestProjectAttributesToParameters_EmptyInputs(t *testing.T) {
	got, _ := projectAttributesToParameters(nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestProjectAttributesToParameters_NoOverlap(t *testing.T) {
	attrs := map[string]interface{}{"foo": "bar"}
	schema := map[string]interface{}{
		"schema": map[string]interface{}{
			"properties": map[string]interface{}{"baz": map[string]interface{}{"type": "string"}},
		},
	}
	got, _ := projectAttributesToParameters(attrs, schema)
	if len(got) != 0 {
		t.Errorf("expected empty map when no keys overlap, got %v", got)
	}
}

func TestProjectAttributesToParameters_CoercesStringValuesToSchemaTypes(t *testing.T) {
	attrs := map[string]interface{}{
		"port":    "6379",
		"enabled": "true",
		"ratio":   "0.75",
		"name":    "redis",
	}
	schema := map[string]interface{}{
		"schema": map[string]interface{}{
			"properties": map[string]interface{}{
				"port":    map[string]interface{}{"type": "integer"},
				"enabled": map[string]interface{}{"type": "boolean"},
				"ratio":   map[string]interface{}{"type": "number"},
				"name":    map[string]interface{}{"type": "string"},
			},
		},
	}
	got, err := projectAttributesToParameters(attrs, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["port"] != int64(6379) {
		t.Errorf("port: got %T(%v), want int64(6379)", got["port"], got["port"])
	}
	if got["enabled"] != true {
		t.Errorf("enabled: got %T(%v), want true", got["enabled"], got["enabled"])
	}
	if got["ratio"] != 0.75 {
		t.Errorf("ratio: got %T(%v), want 0.75", got["ratio"], got["ratio"])
	}
	if got["name"] != "redis" {
		t.Errorf("name: got %v, want \"redis\"", got["name"])
	}
}

func TestProjectAttributesToParameters_CoerceFailureSurfacesError(t *testing.T) {
	attrs := map[string]interface{}{"port": "not-a-number"}
	schema := map[string]interface{}{
		"schema": map[string]interface{}{
			"properties": map[string]interface{}{
				"port": map[string]interface{}{"type": "integer"},
			},
		},
	}
	_, err := projectAttributesToParameters(attrs, schema)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "port") {
		t.Errorf("error %q should mention the failing key", err.Error())
	}
}

func TestProjectAttributesToParameters_CoercesStringToArrayAndObject(t *testing.T) {
	attrs := map[string]interface{}{
		"tags":   `["a","b"]`,
		"config": `{"k":"v"}`,
	}
	schema := map[string]interface{}{
		"schema": map[string]interface{}{
			"properties": map[string]interface{}{
				"tags":   map[string]interface{}{"type": "array"},
				"config": map[string]interface{}{"type": "object"},
			},
		},
	}
	got, err := projectAttributesToParameters(attrs, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags, ok := got["tags"].([]interface{})
	if !ok || len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("tags: got %T(%v), want []interface{}{\"a\",\"b\"}", got["tags"], got["tags"])
	}
	cfg, ok := got["config"].(map[string]interface{})
	if !ok || cfg["k"] != "v" {
		t.Errorf("config: got %T(%v), want map{k:v}", got["config"], got["config"])
	}
}

func TestSummarizeMessages_PrefersLastErrorSeverity(t *testing.T) {
	msgs := []interface{}{
		map[string]interface{}{"severity": "info", "message": "starting"},
		map[string]interface{}{"severity": "error", "message": "first error"},
		map[string]interface{}{"severity": "error", "message": "second error"},
		map[string]interface{}{"severity": "info", "message": "trailing info"},
	}
	got := summarizeMessages(msgs)
	if got != "second error" {
		t.Errorf("got %q, want %q", got, "second error")
	}
}

func TestSummarizeMessages_FallsBackToLastMessage(t *testing.T) {
	msgs := []interface{}{
		map[string]interface{}{"severity": "info", "message": "first"},
		map[string]interface{}{"severity": "info", "message": "last"},
	}
	got := summarizeMessages(msgs)
	if got != "last" {
		t.Errorf("got %q, want %q", got, "last")
	}
}

func TestSummarizeMessages_Empty(t *testing.T) {
	if got := summarizeMessages(nil); got != "no message" {
		t.Errorf("got %q, want %q", got, "no message")
	}
	if got := summarizeMessages([]interface{}{}); got != "no message" {
		t.Errorf("got %q on empty slice, want %q", got, "no message")
	}
}

func TestSummarizeMessages_MalformedEntries(t *testing.T) {
	msgs := []interface{}{
		"not-a-map",
		map[string]interface{}{"severity": "info"},                  // no message
		map[string]interface{}{"message": "ok", "severity": "info"}, // valid, last
	}
	if got := summarizeMessages(msgs); got != "ok" {
		t.Errorf("got %q, want %q", got, "ok")
	}
}

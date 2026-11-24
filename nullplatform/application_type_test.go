package nullplatform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const orderedTypeSchema = `{"type":"object","properties":{"model":{"type":"string"},"autonomy":{"type":"string"}}}`

func TestCreateApplicationType_SendsTheSchemaInItsOrderAndDecodesTheAnswer(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, string(raw)
		_, _ = w.Write([]byte(`{"id":"0b0e","nrn":"organization=1","name":"AI Agent","slug":"ai-agent","description":null,` +
			`"status":"active","visible_to":["organization=1"],"last_snapshot_id":"5f1a","schema":` + orderedTypeSchema + `}`))
	}))
	defer server.Close()

	got, err := newTestClient(server).CreateApplicationType(&ApplicationType{
		Nrn: "organization=1", Name: "AI Agent", Schema: json.RawMessage(orderedTypeSchema),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/application_type" {
		t.Errorf("got %s %s, want POST /application_type", gotMethod, gotPath)
	}
	if want := `{"nrn":"organization=1","name":"AI Agent","schema":` + orderedTypeSchema + `}`; gotBody != want {
		t.Errorf("body:\n got %s\nwant %s", gotBody, want)
	}
	if got.Id != "0b0e" || got.Slug != "ai-agent" || got.LastSnapshotId != "5f1a" || got.Description != nil {
		t.Errorf("unexpected answer: %+v", got)
	}
	if string(got.Schema) != orderedTypeSchema {
		t.Errorf("schema: got %s, want it in the order answered", got.Schema)
	}
}

func TestGetAndPatchApplicationType_Paths(t *testing.T) {
	var calls []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, r.Method+" "+r.URL.Path+" "+string(raw))
		_, _ = w.Write([]byte(`{"id":"0b0e","schema":{}}`))
	}))
	defer server.Close()
	client := newTestClient(server)

	if _, err := client.GetApplicationType("0b0e"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := client.PatchApplicationType("0b0e", &ApplicationType{Status: "inactive"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"GET /application_type/0b0e ", `PATCH /application_type/0b0e {"status":"inactive"}`}
	if strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Errorf("calls:\n got %v\nwant %v", calls, want)
	}
}

func TestApplicationTypeClient_ARefusalCarriesTheResponseBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"The schema is not a valid JSON Schema"}`))
	}))
	defer server.Close()

	_, err := newTestClient(server).PatchApplicationType("0b0e", &ApplicationType{Schema: json.RawMessage(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "status code 400") || !strings.Contains(err.Error(), "The schema is not a valid JSON Schema") {
		t.Errorf("want the status and the API's message, got %v", err)
	}
}

func TestApplicationTypeClient_AnAnswerThatIsNotATypeIsAnError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	if _, err := newTestClient(server).GetApplicationType("0b0e"); err == nil || !strings.Contains(err.Error(), "failed to decode API response") {
		t.Errorf("want a decode error, got %v", err)
	}
}

func TestApplicationTypeClient_ASchemaThatIsNotJSONIsNotSent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("nothing should be sent")
	}))
	defer server.Close()

	_, err := newTestClient(server).CreateApplicationType(&ApplicationType{Schema: json.RawMessage(`{"a":`)})
	if err == nil || !strings.Contains(err.Error(), "failed to encode application type") {
		t.Errorf("want an encode error, got %v", err)
	}
}

func TestApplicationTypeClient_AnAPIThatCannotBeReachedIsAnError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := newTestClient(server)
	server.Close()

	if _, err := client.GetApplicationType("0b0e"); err == nil || !strings.Contains(err.Error(), "failed to make API request") {
		t.Errorf("want a request error, got %v", err)
	}
}

// applicationTypeOps answers the application type calls from memory: the arms
// of the resource a healthy API never reaches.
type applicationTypeOps struct {
	NullOps
	answer *ApplicationType
	err    error
}

func (o applicationTypeOps) CreateApplicationType(*ApplicationType) (*ApplicationType, error) {
	return o.answer, o.err
}

func (o applicationTypeOps) GetApplicationType(string) (*ApplicationType, error) {
	return o.answer, o.err
}

func (o applicationTypeOps) PatchApplicationType(string, *ApplicationType) (*ApplicationType, error) {
	return o.answer, o.err
}

func TestApplicationTypeResource_AnAPIErrorIsReturned(t *testing.T) {
	ops := applicationTypeOps{err: errors.New("failed to get application type: status code 403, response: {\"message\":\"Forbidden\"}")}
	ctx := context.Background()
	data := func() *schema.ResourceData {
		d := schema.TestResourceDataRaw(t, resourceApplicationType().Schema, map[string]any{
			"nrn": "organization=1", "name": "AI Agent", "schema": orderedTypeSchema,
		})
		d.SetId("0b0e")
		return d
	}

	for name, call := range map[string]func(context.Context, *schema.ResourceData, any) (hasError bool, summary string){
		"create": func(ctx context.Context, d *schema.ResourceData, m any) (bool, string) {
			diags := ApplicationTypeCreate(ctx, d, m)
			return diags.HasError(), diags[0].Summary
		},
		"read": func(ctx context.Context, d *schema.ResourceData, m any) (bool, string) {
			diags := ApplicationTypeRead(ctx, d, m)
			return diags.HasError(), diags[0].Summary
		},
		"update": func(ctx context.Context, d *schema.ResourceData, m any) (bool, string) {
			diags := ApplicationTypeUpdate(ctx, d, m)
			return diags.HasError(), diags[0].Summary
		},
		"delete": func(ctx context.Context, d *schema.ResourceData, m any) (bool, string) {
			diags := ApplicationTypeDelete(ctx, d, m)
			return diags.HasError(), diags[0].Summary
		},
	} {
		t.Run(name, func(t *testing.T) {
			d := data()
			hasError, summary := call(ctx, d, ops)
			if !hasError || !strings.Contains(summary, "Forbidden") {
				t.Errorf("want the API's error, got %q", summary)
			}
			// A type that could not be retired is still in the state.
			if name == "delete" && d.Id() != "0b0e" {
				t.Errorf("want the id kept, got %q", d.Id())
			}
		})
	}

	d := schema.TestResourceDataRaw(t, dataSourceApplicationType().Schema, map[string]any{"id": "0b0e"})
	if diags := dataSourceApplicationTypeRead(ctx, d, ops); !diags.HasError() || !strings.Contains(diags[0].Summary, "Forbidden") {
		t.Errorf("data source: want the API's error, got %v", diags)
	}
}

func TestApplicationTypeRead_ASchemaThatIsNotJSONIsAnError(t *testing.T) {
	ops := applicationTypeOps{answer: &ApplicationType{Id: "0b0e", Schema: json.RawMessage(`{"a":`)}}
	d := schema.TestResourceDataRaw(t, resourceApplicationType().Schema, map[string]any{})
	d.SetId("0b0e")

	diags := ApplicationTypeRead(context.Background(), d, ops)
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "application type 0b0e has a schema that is not JSON") {
		t.Errorf("want the schema error, got %v", diags)
	}
}

func TestValidateJSONObject(t *testing.T) {
	for document, valid := range map[string]bool{
		`{"type":"object"}`: true,
		`{}`:                true,
		`[]`:                false,
		`null`:              false,
		`"text"`:            false,
		`{"a":`:             false,
	} {
		_, errs := validateJSONObject(document, "schema")
		if (len(errs) == 0) != valid {
			t.Errorf("validateJSONObject(%s): errors %v, want valid=%v", document, errs, valid)
		}
	}
}

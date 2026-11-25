package nullplatform

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// applicationServer answers GET /application/7 with each of `reads` in turn
// (the last one again once they run out) and everything else with 204, and
// keeps what was asked of it.
func applicationServer(t *testing.T, reads ...string) (*NullClient, *[]string) {
	t.Helper()
	var asked []string
	read := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		asked = append(asked, strings.TrimSpace(r.Method+" "+string(body)))
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		answer := reads[min(read, len(reads)-1)]
		read++
		if status, isStatus := strings.CutPrefix(answer, "status "); isStatus {
			var code int
			_, _ = fmt.Sscan(status, &code)
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"message":"refused"}`))
			return
		}
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	return newTestClient(server), &asked
}

func deleteApplication(t *testing.T, client NullOps) (*schema.ResourceData, error) {
	t.Helper()
	shortenPolling(t)
	d := schema.TestResourceDataRaw(t, resourceApplication().Schema, map[string]any{})
	d.SetId("7")
	if diags := ApplicationDelete(context.Background(), d, client); diags.HasError() {
		return d, fmt.Errorf("%s", diags[0].Summary)
	}
	return d, nil
}

func TestApplicationDelete(t *testing.T) {
	cases := []struct {
		name  string
		reads []string
		asked string
	}{
		{"moves it to deleting and waits for its workflow", []string{`{"id":7,"status":"active"}`, `{"id":7,"status":"deleting"}`, `{"id":7,"status":"deleted"}`},
			`GET|PATCH {"status":"deleting"}|GET|GET`},
		{"takes one that is gone after the wait as deleted", []string{`{"id":7,"status":"failed"}`, "status 404"},
			`GET|PATCH {"status":"deleting"}|GET`},
		{"sends the DELETE it always sent for one already deleting", []string{`{"id":7,"status":"deleting"}`}, `GET|DELETE`},
		{"asks nothing of one already deleted", []string{`{"id":7,"status":"deleted"}`}, `GET`},
		{"asks nothing of one that is gone", []string{"status 404"}, `GET`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, asked := applicationServer(t, c.reads...)
			d, err := deleteApplication(t, client)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := strings.Join(*asked, "|"); got != c.asked {
				t.Errorf("requests: got %s, want %s", got, c.asked)
			}
			if d.Id() != "" {
				t.Errorf("want it out of the state, got id %q", d.Id())
			}
		})
	}
}

func TestApplicationDelete_Failures(t *testing.T) {
	cases := []struct {
		name  string
		reads []string
		want  string
	}{
		{"a delete its workflow fails carries the workflow's message",
			[]string{`{"id":7,"status":"active"}`, `{"id":7,"status":"failed","messages":[{"level":"ERROR","message":"the repository could not be removed"}]}`},
			"the repository could not be removed"},
		{"an application that cannot be read", []string{"status 500"}, "got 500"},
		{"an application that cannot be read while it waits", []string{`{"id":7,"status":"active"}`, "status 500"}, "did not finish"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, _ := applicationServer(t, c.reads...)
			d, err := deleteApplication(t, client)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error with %q, got %v", c.want, err)
			}
			if d.Id() != "7" {
				t.Errorf("want it kept in the state, got id %q", d.Id())
			}
		})
	}
}

// refusingOps is a client whose edits and deletes are refused.
type refusingOps struct {
	NullOps
}

func (refusingOps) PatchApplication(string, *Application) error { return fmt.Errorf("edit refused") }
func (refusingOps) DeleteApplication(string) error              { return fmt.Errorf("delete refused") }

func TestApplicationDelete_ARefusalIsReturned(t *testing.T) {
	for status, want := range map[string]string{"active": "edit refused", "deleting": "delete refused"} {
		client, _ := applicationServer(t, fmt.Sprintf(`{"id":7,"status":%q}`, status))
		if _, err := deleteApplication(t, refusingOps{client}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", status, want, err)
		}
	}
}

func TestPatchApplication_ARefusalCarriesItsDetail(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"The application attributes are invalid","detail":"data/model must be string"}`))
	}))
	defer server.Close()

	err := newTestClient(server).PatchApplication("7", &Application{Attributes: map[string]interface{}{"model": 1}})
	if err == nil || !strings.Contains(err.Error(), "The application attributes are invalid: data/model must be string") {
		t.Errorf("want the message and its detail, got %v", err)
	}
}

// stayingOps is a client whose application never leaves its snapshot.
type stayingOps struct {
	NullOps
	readErr error
}

func (stayingOps) PatchApplication(string, *Application) error { return nil }
func (o stayingOps) GetApplication(string) (*Application, error) {
	return &Application{Id: 7, ApplicationTypeSnapshotId: "snapshot-1"}, o.readErr
}

func TestMoveApplicationToSnapshot_AnApplicationThatStayedIsAnError(t *testing.T) {
	err := moveApplicationToSnapshot(stayingOps{}, "7", &Application{}, "snapshot-2")
	if err == nil || !strings.Contains(err.Error(), "asked to move to snapshot snapshot-2 of its type and is on snapshot-1") {
		t.Errorf("want the snapshot it stayed on, got %v", err)
	}
	if err := moveApplicationToSnapshot(stayingOps{readErr: fmt.Errorf("unreadable")}, "7", &Application{}, "snapshot-2"); err == nil || err.Error() != "unreadable" {
		t.Errorf("want the read's error, got %v", err)
	}
}

func TestSuppressEquivalentJSONOrNoneHeld(t *testing.T) {
	cases := []struct {
		old, new string
		want     bool
	}{
		{`{}`, ``, true},
		{``, ``, true},
		{`{"team":"a"}`, ``, false},
		{`{"team":"a"}`, `{ "team": "a" }`, true},
		{`{}`, `{"team":"a"}`, false},
		{`not json`, ``, false},
	}
	for _, c := range cases {
		if got := suppressEquivalentJSONOrNoneHeld("settings", c.old, c.new, nil); got != c.want {
			t.Errorf("held %q, configured %q: got %v, want %v", c.old, c.new, got, c.want)
		}
	}
}

func TestApplicationUpdate_ARefusalIsReturned(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceApplication().Schema, map[string]any{"name": "my-api", "namespace_id": 3})
	d.SetId("7")

	diags := ApplicationUpdate(context.Background(), d, refusingOps{})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "edit refused") {
		t.Errorf("want the refusal, got %v", diags)
	}
}

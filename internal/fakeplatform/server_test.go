package fakeplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// A create after Seed takes the next id past the seeded one, never the seeded
// one itself, in both id shapes.
func TestSeedMovesTheIDSequence(t *testing.T) {
	s := New()
	defer s.Close()
	s.RegisterWith("numeric", "n", Options{NumericIDs: true}, Hooks{})
	s.Register("prefixed", "p", Hooks{})
	s.Seed("numeric", "7", Item{"name": "seeded"})
	s.Seed("prefixed", "p-3", Item{"name": "seeded"})

	for _, tc := range []struct{ collection, want string }{{"numeric", "8"}, {"prefixed", "p-4"}} {
		res, err := s.ts.Client().Post(s.URL()+"/"+tc.collection, "application/json", strings.NewReader(`{"name":"created"}`))
		if err != nil {
			t.Fatal(err)
		}
		var created Item
		_ = json.NewDecoder(res.Body).Decode(&created)
		res.Body.Close()
		if res.StatusCode != http.StatusOK || idString(created) != tc.want {
			t.Errorf("%s: create answered %d with id %v, want %s", tc.collection, res.StatusCode, created["id"], tc.want)
		}
		if len(s.Items(tc.collection)) != 2 {
			t.Errorf("%s: %d items, want the seeded one and the created one", tc.collection, len(s.Items(tc.collection)))
		}
	}
}

// OnRequest sees every segment after the collection's name, answers a nested
// route with a body (unescaped) or without one, and a nil answer falls through
// to the generic routes.
func TestOnRequestAnswersOrFallsThrough(t *testing.T) {
	s := New()
	defer s.Close()
	var seen []string
	s.Register("things", "t", Hooks{OnRequest: func(_ *Server, method string, segments []string, body []byte) *Answer {
		seen = append(seen, method+" "+strings.Join(segments, "|"))
		switch {
		case len(segments) == 4 && segments[1] == "a":
			return &Answer{Status: http.StatusAccepted, Body: Item{"echo": string(body), "path": segments}}
		case len(segments) == 2 && method == http.MethodDelete:
			return &Answer{Status: http.StatusNoContent}
		}
		return nil
	}})
	do := func(method, path, body string) (int, string) {
		req, _ := http.NewRequest(method, s.URL()+path, strings.NewReader(body))
		res, err := s.ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var buf strings.Builder
		_, _ = io.Copy(&buf, res.Body)
		return res.StatusCode, buf.String()
	}

	status, out := do(http.MethodPost, "/things/x/a/b/c", "<&>")
	if status != http.StatusAccepted || !strings.Contains(out, `"echo":"<&>"`) || !strings.Contains(out, `"path":["x","a","b","c"]`) {
		t.Errorf("nested route answered %d %s", status, out)
	}
	if status, out = do(http.MethodDelete, "/things/x/y", ""); status != http.StatusNoContent || out != "" {
		t.Errorf("bodiless answer was %d %q", status, out)
	}
	// Nil: the generic CRUD runs as if there were no hook.
	status, out = do(http.MethodPost, "/things", `{"name":"n"}`)
	if status != http.StatusOK || !strings.Contains(out, `"id":"t-1"`) {
		t.Errorf("fall-through create answered %d %s", status, out)
	}
	if status, _ = do(http.MethodGet, "/things/t-1", ""); status != http.StatusOK {
		t.Errorf("fall-through get answered %d", status)
	}
	if len(s.Items("things")) != 1 || len(seen) != 4 || seen[2] != "POST " {
		t.Errorf("items %d, seen %v", len(s.Items("things")), seen)
	}
}

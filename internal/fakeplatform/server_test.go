package fakeplatform

import (
	"encoding/json"
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

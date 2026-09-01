// Package fakeplatform is a hermetic, STATEFUL fake of the nullplatform API
// for functional tests: a generic in-memory REST engine plus per-resource
// behavior hooks. The engine makes emulating a new endpoint family a
// three-line registration; the hooks are where a contract (guards, refusals,
// asynchronous transitions) is encoded, one file per behavior area.
//
// Items are raw JSON objects (map[string]any), so the fake needs no schema to
// echo whatever a client stores — only the fields a behavior hook inspects are
// ever named. Every rule a hook enforces must be a behavior observable against
// the real API; the fake is the provider's executable copy of that contract,
// and `make testacc` remains the on-demand check that the real API agrees.
package fakeplatform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// Item is a stored resource, raw as the client sent it.
type Item = map[string]any

// Refusal is a guard rejection: the HTTP status and the message the real API
// would answer — the message IS contract, providers surface it to operators.
type Refusal struct {
	Status  int
	Message string
}

func Refuse(format string, args ...any) *Refusal {
	return &Refusal{Status: http.StatusBadRequest, Message: fmt.Sprintf(format, args...)}
}

// Hooks are a collection's behavior. Every hook is optional; a nil hook means
// plain generic CRUD. Hooks run under the server lock and may inspect other
// collections through the Server.
type Hooks struct {
	// OnCreate may refuse or mutate the item about to be stored.
	OnCreate func(s *Server, item Item) *Refusal
	// OnPatch applies a patch to an existing item; returning a Refusal leaves
	// the item untouched. When nil, the patch is shallow-merged.
	OnPatch func(s *Server, existing, patch Item) *Refusal
	// OnGet runs before an item is returned — the place asynchronous
	// transitions progress, one observation at a time.
	OnGet func(s *Server, item Item)
	// OnDelete may refuse the hard delete.
	OnDelete func(s *Server, item Item) *Refusal
}

type Options struct {
	NumericIDs   bool
	CreateStatus int
}

type collection struct {
	prefix string
	items  map[string]Item
	seq    int
	opts   Options
	hooks  Hooks
}

// Server is the fake platform: one instance per test.
type Server struct {
	mu          sync.Mutex
	collections map[string]*collection
	gets        map[string]int
	Deletes     int
	ts          *httptest.Server
}

func New() *Server {
	s := &Server{
		collections: map[string]*collection{},
		gets:        map[string]int{},
	}
	s.ts = httptest.NewTLSServer(http.HandlerFunc(s.handle))
	return s
}

func (s *Server) Close()                 { s.ts.Close() }
func (s *Server) URL() string            { return s.ts.URL }
func (s *Server) HTTP() *httptest.Server { return s.ts }

// Register mounts a collection at /<name> with generic CRUD + the hooks.
// idPrefix names generated ids ("svc" -> svc-1, svc-2, ...).
func (s *Server) Register(name, idPrefix string, hooks Hooks) {
	s.RegisterWith(name, idPrefix, Options{}, hooks)
}

func (s *Server) RegisterWith(name, idPrefix string, opts Options, hooks Hooks) {
	s.collections[name] = &collection{prefix: idPrefix, items: map[string]Item{}, opts: opts, hooks: hooks}
}

// Items returns every stored item of a collection (for CheckDestroy-style
// assertions). The caller must not mutate concurrently with requests.
func (s *Server) Items(name string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Item
	for _, item := range s.collections[name].items {
		out = append(out, item)
	}
	return out
}

// Seed stores an item directly, bypassing hooks — test arrangement, not API
// behavior.
func (s *Server) Seed(name, id string, item Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item["id"] = id
	s.collections[name].items[id] = item
}

// Gets reports how many times an item has been observed — behavior hooks use
// it to progress asynchronous transitions; ResetGets restarts the count when a
// new transition begins.
func (s *Server) Gets(id string) int  { return s.gets[id] }
func (s *Server) ResetGets(id string) { s.gets[id] = 0 }

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/")
	name, id, _ := strings.Cut(path, "/")
	col, ok := s.collections[name]
	if !ok {
		http.Error(w, fmt.Sprintf(`{"message":"fakeplatform: no collection %q registered"}`, name), http.StatusInternalServerError)
		return
	}

	switch {
	case r.Method == http.MethodPost && id == "":
		item := Item{}
		_ = json.NewDecoder(r.Body).Decode(&item)
		if col.hooks.OnCreate != nil {
			if refusal := col.hooks.OnCreate(s, item); refusal != nil {
				writeRefusal(w, refusal)
				return
			}
		}
		col.seq++
		id := fmt.Sprintf("%s-%d", col.prefix, col.seq)
		if col.opts.NumericIDs {
			id = strconv.Itoa(col.seq)
			item["id"] = col.seq
		} else {
			item["id"] = id
		}
		col.items[id] = item
		status := col.opts.CreateStatus
		if status == 0 {
			status = http.StatusOK
		}
		writeJSONStatus(w, status, item)

	case r.Method == http.MethodGet && id != "":
		item, ok := col.items[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		s.gets[id]++
		if col.hooks.OnGet != nil {
			col.hooks.OnGet(s, item)
		}
		writeJSON(w, item)

	case r.Method == http.MethodPatch && id != "":
		item, ok := col.items[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		patch := Item{}
		_ = json.NewDecoder(r.Body).Decode(&patch)
		if col.hooks.OnPatch != nil {
			if refusal := col.hooks.OnPatch(s, item, patch); refusal != nil {
				writeRefusal(w, refusal)
				return
			}
		} else {
			for key, value := range patch {
				item[key] = value
			}
		}
		writeJSON(w, item)

	case r.Method == http.MethodDelete && id != "":
		item, ok := col.items[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		if col.hooks.OnDelete != nil {
			if refusal := col.hooks.OnDelete(s, item); refusal != nil {
				writeRefusal(w, refusal)
				return
			}
		}
		s.Deletes++
		delete(col.items, id)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, `{"message":"fakeplatform: unhandled route"}`, http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeRefusal(w http.ResponseWriter, refusal *Refusal) {
	w.WriteHeader(refusal.Status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": refusal.Message})
}

// Str reads a string field of an item, "" when absent.
func Str(item Item, key string) string {
	v, _ := item[key].(string)
	return v
}

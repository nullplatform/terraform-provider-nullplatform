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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Item is a stored resource, raw as the client sent it.
type Item = map[string]any

// Refusal is a guard rejection: the HTTP status and the message the real API
// would answer — the message IS contract, providers surface it to operators.
// Body, when set, is the whole error body the API answers instead of
// {"message"}: its other members are contract too (a numeric "status" breaks
// a client that decodes it as a string).
type Refusal struct {
	Status  int
	Message string
	Body    Item
}

// Request is one call the fake received, exactly as the client sent it: the
// raw query, and the body's raw bytes, trailing newline included.
type Request struct {
	Method string
	Path   string
	Query  string
	Body   string
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
	// OnPatchNew, when set, answers a PATCH instead of OnPatch, for an API
	// whose PATCH leaves the item untouched and inserts a new one (a new
	// version): it returns that item, which the engine stores under a fresh id
	// and answers.
	OnPatchNew func(s *Server, existing, patch Item) (Item, *Refusal)
	// OnList answers GET /<collection>, whole body included; nil leaves the
	// route unhandled. Query() has the request's filters.
	OnList func(s *Server) (any, *Refusal)
	// OnGet runs before an item is returned — the place asynchronous
	// transitions progress, one observation at a time.
	OnGet func(s *Server, item Item)
	// OnDelete may refuse the delete; with Options.KeepDeleted it is the
	// delete itself (a soft delete that marks the item).
	OnDelete func(s *Server, item Item) *Refusal
	// OnMissing answers a request for an id the collection does not hold;
	// nil keeps the generic 404.
	OnMissing func(s *Server, id string) *Refusal
	// OnSub answers the sub-resources an item owns,
	// /<collection>/<id>/<sub>[/<subID>] (associations, links): it gets the
	// method, the sub-resource's name, its id ("" when the path ends at the
	// name) and the decoded body, and returns the success status (answered
	// without a body) or a Refusal. Status 0 is an unhandled route.
	OnSub func(s *Server, parent Item, method, sub, subID string, body Item) (int, *Refusal)
}

type Options struct {
	NumericIDs   bool
	CreateStatus int
	// PatchStatus http.StatusNoContent answers a PATCH with 204 and no body,
	// as an API that does; zero answers 200 with the item.
	PatchStatus int
	// KeepDeleted keeps an item after a DELETE: OnDelete marks it, as the
	// API's soft delete does, and it stays readable.
	KeepDeleted bool
	// IDFormat mints ids from the sequence number ("spec_%016d"); empty mints
	// <idPrefix>-<n>. NumericIDs wins over it.
	IDFormat string
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
	requests    []Request
	query       url.Values
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
// idPrefix names generated ids ("svc" -> svc-1, svc-2, ...). A name may span
// segments ("parent/child"); a request goes to the longest registered name
// its path starts with.
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
// behavior. A NumericIDs collection stores the id as a number, as it answers.
// The id sequence moves past a seeded id, so a later create never lands on it.
func (s *Server) Seed(name, id string, item Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	col := s.collections[name]
	item["id"] = id
	if n, err := strconv.Atoi(id); err == nil && col.opts.NumericIDs {
		item["id"] = n
	}
	prefix := strings.TrimRightFunc(id, func(r rune) bool { return r >= '0' && r <= '9' })
	if n, err := strconv.Atoi(id[len(prefix):]); err == nil && col.idFor(n) == id && n > col.seq {
		col.seq = n
	}
	col.items[id] = item
}

// idFor is the id the collection mints for sequence number n.
func (c *collection) idFor(n int) string {
	switch {
	case c.opts.NumericIDs:
		return strconv.Itoa(n)
	case c.opts.IDFormat != "":
		return fmt.Sprintf(c.opts.IDFormat, n)
	}
	return fmt.Sprintf("%s-%d", c.prefix, n)
}

// store keeps a new item under the next id.
func (c *collection) store(item Item) {
	c.seq++
	id := c.idFor(c.seq)
	item["id"] = id
	if c.opts.NumericIDs {
		item["id"] = c.seq
	}
	c.items[id] = item
}

// Query is the query of the request being answered, for a hook to read: hooks
// run under the lock, one request at a time.
func (s *Server) Query() url.Values { return s.query }

// Requests returns every request received so far, in arrival order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Gets reports how many times an item has been observed — behavior hooks use
// it to progress asynchronous transitions; ResetGets restarts the count when a
// new transition begins.
func (s *Server) Gets(id string) int  { return s.gets[id] }
func (s *Server) ResetGets(id string) { s.gets[id] = 0 }

// route resolves a path to the longest registered collection it starts with
// and the segments after it: [], [id] or [id, sub[, subID]].
func (s *Server) route(path string) (string, []string) {
	name := ""
	for registered := range s.collections {
		if (path == registered || strings.HasPrefix(path, registered+"/")) && len(registered) > len(name) {
			name = registered
		}
	}
	if name == "" {
		name, _, _ = strings.Cut(path, "/")
		return name, nil
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(path, name), "/")
	if rest == "" {
		return name, nil
	}
	return name, strings.SplitN(rest, "/", 3)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	s.requests = append(s.requests, Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(raw)})
	s.query = r.URL.Query()
	body := func(v any) { _ = json.NewDecoder(bytes.NewReader(raw)).Decode(v) }

	name, segments := s.route(strings.TrimPrefix(r.URL.Path, "/"))
	col, ok := s.collections[name]
	if !ok {
		http.Error(w, fmt.Sprintf(`{"message":"fakeplatform: no collection %q registered"}`, name), http.StatusInternalServerError)
		return
	}
	id := ""
	if len(segments) > 0 {
		id = segments[0]
	}
	if _, held := col.items[id]; id != "" && !held {
		if col.hooks.OnMissing != nil {
			if refusal := col.hooks.OnMissing(s, id); refusal != nil {
				writeRefusal(w, refusal)
				return
			}
		}
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
		return
	}

	switch {
	case len(segments) > 1 && col.hooks.OnSub != nil:
		sub := Item{}
		body(&sub)
		subID := ""
		if len(segments) > 2 {
			subID = segments[2]
		}
		status, refusal := col.hooks.OnSub(s, col.items[id], r.Method, segments[1], subID, sub)
		switch {
		case refusal != nil:
			writeRefusal(w, refusal)
		case status == 0:
			http.Error(w, `{"message":"fakeplatform: unhandled route"}`, http.StatusInternalServerError)
		default:
			w.WriteHeader(status)
		}

	case len(segments) > 1:
		http.Error(w, `{"message":"fakeplatform: unhandled route"}`, http.StatusInternalServerError)

	case r.Method == http.MethodGet && id == "" && col.hooks.OnList != nil:
		list, refusal := col.hooks.OnList(s)
		if refusal != nil {
			writeRefusal(w, refusal)
			return
		}
		writeJSON(w, list)

	case r.Method == http.MethodPatch && id != "" && col.hooks.OnPatchNew != nil:
		patch := Item{}
		body(&patch)
		created, refusal := col.hooks.OnPatchNew(s, col.items[id], patch)
		if refusal != nil {
			writeRefusal(w, refusal)
			return
		}
		col.store(created)
		writeJSON(w, created)

	case r.Method == http.MethodPost && id == "":
		item := Item{}
		body(&item)
		if col.hooks.OnCreate != nil {
			if refusal := col.hooks.OnCreate(s, item); refusal != nil {
				writeRefusal(w, refusal)
				return
			}
		}
		col.store(item)
		status := col.opts.CreateStatus
		if status == 0 {
			status = http.StatusOK
		}
		writeJSONStatus(w, status, item)

	case r.Method == http.MethodGet && id != "":
		item := col.items[id]
		s.gets[id]++
		if col.hooks.OnGet != nil {
			col.hooks.OnGet(s, item)
		}
		writeJSON(w, item)

	case r.Method == http.MethodPatch && id != "":
		item := col.items[id]
		patch := Item{}
		body(&patch)
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
		if col.opts.PatchStatus == http.StatusNoContent {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, item)

	case r.Method == http.MethodDelete && id != "":
		item := col.items[id]
		if col.hooks.OnDelete != nil {
			if refusal := col.hooks.OnDelete(s, item); refusal != nil {
				writeRefusal(w, refusal)
				return
			}
		}
		s.Deletes++
		if !col.opts.KeepDeleted {
			delete(col.items, id)
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, `{"message":"fakeplatform: unhandled route"}`, http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

// writeJSONStatus answers as the API does: no HTML escaping.
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeRefusal(w http.ResponseWriter, refusal *Refusal) {
	if refusal.Body != nil {
		writeJSONStatus(w, refusal.Status, refusal.Body)
		return
	}
	writeJSONStatus(w, refusal.Status, map[string]string{"message": refusal.Message})
}

// Str reads a string field of an item, "" when absent.
func Str(item Item, key string) string {
	v, _ := item[key].(string)
	return v
}

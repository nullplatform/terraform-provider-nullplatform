package fakeplatform

// The workflow engine contract as behavior hooks, messages and types literal.
// A rule marked "(observed against the API)" was answered by the real API; one
// marked "(read from the API's code)" was read from its source; one marked
// "(fake's choice)" is not known and is only what the fake needs to be
// coherent. Not emulated, because the provider never sends it: a plugin's
// config validation against its descriptor (a 400 CONFIG_INVALID), the
// webhook endpoint itself, a definitions listing and the engine's permission
// checks.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	// WorkflowDefinitions is the collection holding workflows, keyed by their
	// wf_ id; each item holds its record ("workflow"), its revisions
	// ("revisions", []Item) and its aliases ("aliases", map[string]Item).
	WorkflowDefinitions = "workflows/definitions"
	// NotificationChannels is the collection a checklist trigger's channel
	// lives in (numeric ids).
	NotificationChannels = "notification/channel"
)

// WorkflowTimestamp is every timestamp the fake answers.
const WorkflowTimestamp = "2026-01-01T00:00:00.000Z"

const (
	errorsBase        = "https://workflow-system.dev/errors/"
	checklistTrigger  = "np-checklist-trigger"
	defaultChecklist  = "checklist:item:dispatched"
	defaultWorkflowNR = "organization=1"
)

var (
	workflowKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	stepIDPattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	nrnPattern         = regexp.MustCompile(`^organization=[0-9]+(:[a-z]+=[0-9]+)*$`)
	slugPattern        = regexp.MustCompile(`[^a-z0-9]+`)
)

// WorkflowEngine is how the engine behaves in one test, declared the way the
// real engine decides: by the channel a trigger registers, at an NRN.
type WorkflowEngine struct {
	// RefuseActiveAliasDelete: a DELETE of an active alias answers 409
	// alias-active (otherwise 204, the alias gone, its triggers left
	// registered, as the API answers when it does not refuse).
	RefuseActiveAliasDelete bool
	// ReportReleaseFailures with UnreleasableChannelNRNs: a deactivation that
	// cannot delete a channel at one of those NRNs answers 200 with
	// deactivateError and keeps the channel. Without the flag it keeps the
	// channel silently, no error.
	ReportReleaseFailures   bool
	UnreleasableChannelNRNs []string
	// ReportActivationError with DeniedChannelNRNs: an activation whose
	// checklist trigger creates its channel at one of those NRNs fails (200,
	// failedTriggers with that trigger, alias.active false); with the flag the
	// answer also carries activationError.
	ReportActivationError bool
	DeniedChannelNRNs     []string
	// UnreleasedOnDelete: a DELETE of an inactive alias (or of a workflow
	// holding one) that still has an unreleased channel answers 409
	// alias-not-released.
	UnreleasedOnDelete bool
}

type workflowFake struct{ engine *WorkflowEngine }

// RegisterWorkflows mounts /workflows/definitions and /notification/channel
// with engine as its starting behavior. SetWorkflowEngine changes it later.
func RegisterWorkflows(s *Server, engine WorkflowEngine) {
	w := &workflowFake{engine: &engine}
	s.RegisterWith(WorkflowDefinitions, "wf", Options{IDFormat: "wf_%012d"}, Hooks{OnRequest: w.onRequest})
	s.RegisterWith(NotificationChannels, "ch", Options{NumericIDs: true, CreateStatus: http.StatusCreated}, Hooks{OnList: listChannels})
	s.workflow = w
}

// SetWorkflowEngine changes the engine's behavior under the server's lock,
// which every request holds: a test lifts a refusal between steps without a
// data race. change gets the live value and must not keep it.
func SetWorkflowEngine(s *Server, change func(*WorkflowEngine)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	change(s.workflow.engine)
}

// WorkflowChannels returns the channels that exist now, by id.
func WorkflowChannels(s *Server) []Item {
	items := s.Items(NotificationChannels)
	sort.Slice(items, func(i, j int) bool { return numericID(items[i]) < numericID(items[j]) })
	return items
}

// WorkflowRevisions returns a copy of a workflow's revisions, oldest first,
// nil when the workflow does not exist.
func WorkflowRevisions(s *Server, workflowID string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.collections[WorkflowDefinitions].items[workflowID]
	if !ok {
		return nil
	}
	var out []Item
	for _, revision := range revisions(item) {
		out = append(out, jsonCopy(revision).(map[string]any))
	}
	return out
}

// WorkflowAlias returns a copy of a workflow's alias, triggerStates included.
func WorkflowAlias(s *Server, workflowID, name string) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.collections[WorkflowDefinitions].items[workflowID]
	if !ok {
		return nil, false
	}
	alias, ok := item["aliases"].(map[string]Item)[name]
	if !ok {
		return nil, false
	}
	return jsonCopy(alias).(Item), true
}

func listChannels(s *Server) (any, *Refusal) {
	q := s.Query()
	var matching []Item
	for _, item := range s.collections[NotificationChannels].items {
		if nrn := q.Get("nrn"); nrn == "" || Str(item, "nrn") == nrn {
			matching = append(matching, item)
		}
	}
	sort.Slice(matching, func(i, j int) bool { return numericID(matching[i]) < numericID(matching[j]) })
	limit, offset := 100, 0
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = n
	}
	if n, err := strconv.Atoi(q.Get("offset")); err == nil && n > 0 {
		offset = n
	}
	page := []Item{}
	for i := offset; i < len(matching) && i < offset+limit; i++ {
		page = append(page, matching[i])
	}
	return Item{"paging": Item{"limit": limit, "offset": offset, "total": len(matching)}, "results": page}, nil
}

func numericID(item Item) int { n, _ := toInt(item["id"]); return n }

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), n == float64(int(n))
	case json.Number:
		i, err := strconv.Atoi(n.String())
		return i, err == nil
	}
	return 0, false
}

func jsonCopy(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	_ = dec.Decode(&out)
	return out
}

// problemBody is an error as the engine answers it (observed against the
// API): application/problem+json, type https://workflow-system.dev/errors/<slug>.
// The titles are the fake's choice.
func problemAnswer(status int, slug, title, detail, instance string, errs ...Item) *Answer {
	body := Item{"type": errorsBase + slug, "title": title, "status": status, "detail": detail, "instance": instance}
	if len(errs) > 0 {
		body["errors"] = errs
	}
	return &Answer{Status: status, Body: body}
}

// codeAnswer is the other 400 shape (observed against the API):
// {"error": {"code", "message"}}.
func codeAnswer(code, message string) *Answer {
	return &Answer{Status: http.StatusBadRequest, Body: Item{"error": Item{"code": code, "message": message}}}
}

func notFound(ref, instance string) *Answer {
	return problemAnswer(404, "workflow-not-found", "Workflow not found", fmt.Sprintf("No workflow '%s' exists", ref), instance)
}

// A missing workflow under an alias route answers another detail (observed
// against the API).
func notFoundForAlias(id, instance string) *Answer {
	return problemAnswer(404, "workflow-not-found", "Workflow not found", fmt.Sprintf("No workflow with id %s exists", id), instance)
}

func aliasNotFound(id, name, instance string) *Answer {
	return problemAnswer(404, "alias-not-found", "Alias not found", fmt.Sprintf("No alias '%s' on workflow %s", name, id), instance) // observed against the API
}

func revisionNotFound(id string, revision int, instance string) *Answer {
	return problemAnswer(404, "revision-not-found", "Revision not found", fmt.Sprintf("No revision %d on workflow %s", revision, id), instance) // observed against the API
}

func invalid(detail, instance string) *Answer {
	return problemAnswer(400, "validation-failed", "Validation failed", detail, instance)
}

func decodeBody(raw []byte) (Item, bool) {
	body := Item{}
	if len(bytes.TrimSpace(raw)) == 0 {
		return body, true
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return body, dec.Decode(&body) == nil
}

// onRequest routes /workflows/definitions[/<ref>[/aliases[/<a>[/<action>]]]].
func (w *workflowFake) onRequest(s *Server, method string, seg []string, raw []byte) *Answer {
	instance := "/" + WorkflowDefinitions
	if len(seg) > 0 {
		instance += "/" + strings.Join(seg, "/")
	}
	body, ok := decodeBody(raw)
	if !ok && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
		return invalid("request body is not valid JSON", instance)
	}
	col := s.collections[WorkflowDefinitions]
	switch {
	case len(seg) == 0 && method == http.MethodPost:
		return w.create(col, body, "", instance)
	case len(seg) == 1:
		return w.definition(s, col, method, seg[0], body, instance)
	case len(seg) >= 2 && seg[1] == "aliases" && len(seg) <= 4:
		item := lookup(col, seg[0])
		if item == nil {
			return notFoundForAlias(seg[0], instance)
		}
		return w.aliasRoute(s, item, method, seg[2:], body, instance)
	}
	return nil
}

func lookup(col *collection, ref string) Item {
	if item, ok := col.items[ref]; ok {
		return item
	}
	for _, item := range col.items {
		if key := Str(record(item), "key"); key != "" && key == ref {
			return item
		}
	}
	return nil
}

func record(item Item) Item             { return item["workflow"].(Item) }
func revisions(item Item) []Item        { return item["revisions"].([]Item) }
func aliases(item Item) map[string]Item { return item["aliases"].(map[string]Item) }

func (w *workflowFake) definition(s *Server, col *collection, method, ref string, body Item, instance string) *Answer {
	item := lookup(col, ref)
	switch method {
	case http.MethodGet:
		if item == nil {
			return notFound(ref, instance)
		}
		return w.read(item, s.Query(), instance)
	case http.MethodPut:
		def, _ := body["definition"].(map[string]any)
		if item == nil {
			// A PUT to an unknown valid key creates the workflow (201, mode
			// created); to an unknown wf_ id, 404 (observed against the API).
			if strings.HasPrefix(ref, "wf_") || !workflowKeyPattern.MatchString(ref) {
				return notFound(ref, instance)
			}
			if def == nil {
				return invalid("definition is required", instance)
			}
			def["key"] = ref
			answer := w.create(col, Item(def), ref, instance)
			if answer.Status == http.StatusCreated {
				answer.Body.(Item)["mode"] = "created"
			}
			return answer
		}
		return w.newRevision(item, def, Str(body, "mode"), instance)
	case http.MethodPatch:
		if item == nil {
			return notFound(ref, instance)
		}
		return w.patch(col, item, body, instance)
	case http.MethodDelete:
		if item == nil {
			return notFound(ref, instance)
		}
		return w.delete(s, col, item, instance)
	}
	return nil
}

// create handles POST (and the PUT that creates).
func (w *workflowFake) create(col *collection, body Item, forcedKey, instance string) *Answer {
	name, _ := body["name"].(string)
	if name == "" {
		return codeAnswer("NAME_REQUIRED", `Workflow "name" is required.`) // read from the API's code
	}
	key, _ := body["key"].(string)
	if body["key"] != nil && !workflowKeyPattern.MatchString(key) {
		return codeAnswer("KEY_INVALID", `Workflow "key" must match /^[a-z0-9][a-z0-9-]{0,62}$/ (lowercase slug, ≤63 chars).`) // read from the API's code
	}
	nrn := defaultWorkflowNR
	if value, present := body["nrn"]; present && value != nil {
		nrn, _ = value.(string)
		if !nrnPattern.MatchString(nrn) {
			return codeAnswer("NRN_INVALID", "nrn is not valid") // observed against the API
		}
	}
	org := organizationOf(nrn)
	if key != "" {
		for _, other := range col.items {
			if Str(record(other), "key") == key && organizationOf(Str(record(other), "nrn")) == org {
				return problemAnswer(409, "key-already-exists", "Key already exists", fmt.Sprintf("a workflow with key '%s' already exists", key), instance) // fake's choice: the API's uniqueness answer was not observed
			}
		}
	}
	id := col.idFor(col.seq + 1)
	definition, refusal := buildDefinition(body, id, instance)
	if refusal != nil {
		return refusal
	}
	rec := Item{"id": id, "name": name, "nrn": nrn, "organizationId": org, "createdAt": WorkflowTimestamp, "updatedAt": WorkflowTimestamp}
	for _, k := range []string{"key", "description", "path"} {
		if value, ok := body[k].(string); ok {
			rec[k] = value
		}
	}
	if value, ok := body["metadata"]; ok {
		rec["metadata"] = value // copied on create, a PUT does not update it (observed against the API)
	}
	item := Item{"workflow": rec, "revisions": []Item{revisionOf(id, 1, definition)}, "aliases": map[string]Item{}}
	col.store(item)
	refreshLatest(item)
	return &Answer{Status: http.StatusCreated, Body: Item{"workflow": rec, "revision": revisions(item)[0], "latestAlias": aliases(item)["latest"]}}
}

func organizationOf(nrn string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(strings.SplitN(nrn, ":", 2)[0], "organization="))
	return n
}

func revisionOf(id string, n int, definition Item) Item {
	return Item{"workflowId": id, "revision": n, "definition": definition, "createdAt": WorkflowTimestamp, "createdBy": "fake-user"}
}

// refreshLatest keeps the "latest" alias at the highest revision (observed
// against the API: the engine maintains it).
func refreshLatest(item Item) {
	revs := revisions(item)
	aliases(item)["latest"] = Item{"workflowId": Str(record(item), "id"), "name": "latest", "revision": len(revs), "active": false, "updatedAt": WorkflowTimestamp, "updatedBy": "fake-user"}
}

// isTriggerPlugin approximates the API's plugin registry: the fake knows no
// descriptors, so a plugin is a trigger plugin when its name is "webhook" or
// ends in "-trigger".
func isTriggerPlugin(plugin string) bool {
	return plugin == "webhook" || strings.HasSuffix(plugin, "-trigger")
}

// buildDefinition is what the engine stores for a definition as sent (observed
// against the API): the body as sent, plus id, triggers (rebuilt from the
// steps), pinnedDescriptors and each step's type when it had none. Nothing
// else changes, record keys sent inside it included.
func buildDefinition(sent Item, id, instance string) (Item, *Answer) {
	definition := jsonCopy(sent).(map[string]any)
	definition["id"] = id
	triggers := []any{}
	pinned := Item{}
	if steps, ok := definition["steps"].(map[string]any); ok {
		ids := make([]string, 0, len(steps))
		for stepID := range steps {
			ids = append(ids, stepID)
		}
		sort.Strings(ids)
		var bad []Item
		for _, stepID := range ids {
			if !stepIDPattern.MatchString(stepID) {
				bad = append(bad, Item{"code": "STEP_ID_INVALID_FORMAT", "message": fmt.Sprintf("step id '%s' must match %s", stepID, stepIDPattern), "path": "steps." + stepID}) // observed against the API
			}
		}
		if len(bad) > 0 {
			return nil, problemAnswer(400, "graph-invalid", "Workflow graph validation failed", fmt.Sprintf("%d error(s) in workflow definition", len(bad)), instance, bad...)
		}
		for _, stepID := range ids {
			step, ok := steps[stepID].(map[string]any)
			if !ok {
				continue
			}
			plugin, _ := step["pluginType"].(string)
			if t, _ := step["type"].(string); t == "" {
				step["type"] = "module"
				if isTriggerPlugin(plugin) {
					step["type"] = "trigger"
				}
			}
			if plugin != "" {
				pinned[step["type"].(string)+":"+plugin] = Item{"inputPorts": []any{}, "outputPorts": []any{}} // observed against the API: keyed <type>:<plugin>; the values are the fake's choice
			}
			if step["type"] == "trigger" {
				config, ok := step["config"]
				if !ok {
					config = Item{}
				}
				// {id, pluginType, mode, config} (observed against the API).
				trigger := map[string]any{"id": stepID, "pluginType": plugin, "mode": "start", "config": config}
				triggers = append(triggers, trigger)
			}
		}
	}
	definition["triggers"], definition["pinnedDescriptors"] = triggers, pinned
	return definition, nil
}

func (w *workflowFake) read(item Item, q url.Values, instance string) *Answer {
	rec, revs := record(item), revisions(item)
	id := Str(rec, "id")
	switch {
	case q.Get("revision") != "":
		n, _ := strconv.Atoi(q.Get("revision"))
		if n < 1 || n > len(revs) {
			return revisionNotFound(id, n, instance)
		}
		return &Answer{Status: 200, Body: Item{"workflow": rec, "revision": revs[n-1], "resolvedVia": "revision"}}
	case q.Get("alias") != "":
		alias, ok := aliases(item)[q.Get("alias")]
		if !ok {
			return aliasNotFound(id, q.Get("alias"), instance)
		}
		n, _ := toInt(alias["revision"])
		return &Answer{Status: 200, Body: Item{"workflow": rec, "revision": revs[n-1], "resolvedVia": "alias", "aliasName": q.Get("alias")}}
	}
	return &Answer{Status: 200, Body: Item{"workflow": rec, "revision": revs[len(revs)-1], "resolvedVia": "latest"}}
}

// newRevision is PUT {definition, mode} on an existing workflow.
func (w *workflowFake) newRevision(item Item, def map[string]any, mode, instance string) *Answer {
	rec, id := record(item), Str(record(item), "id")
	if def == nil {
		return invalid("definition is required", instance)
	}
	if name, _ := def["name"].(string); name == "" {
		// observed against the API
		return problemAnswer(400, "graph-invalid", "Workflow graph validation failed", "1 error(s) in workflow definition", instance,
			Item{"code": "WORKFLOW_NAME_MISSING", "message": "Workflow name is required and must be a non-empty string.", "path": "name"})
	}
	definition, refusal := buildDefinition(def, id, instance)
	if refusal != nil {
		return refusal
	}
	if nrn, ok := def["nrn"].(string); ok {
		if !nrnPattern.MatchString(nrn) {
			return codeAnswer("NRN_INVALID", "nrn is not valid")
		}
		rec["nrn"] = nrn // the update path changes no record field but nrn (observed against the API)
	}
	revs := revisions(item)
	overwrite := false
	if mode == "" || mode == "auto" {
		// auto overwrites the latest revision unless an active alias points at it
		// (observed against the API).
		overwrite = true
		for name, alias := range aliases(item) {
			if n, _ := toInt(alias["revision"]); name != "latest" && alias["active"] == true && n == len(revs) {
				overwrite = false
			}
		}
	}
	if overwrite {
		revs[len(revs)-1]["definition"] = definition
	} else {
		item["revisions"] = append(revs, revisionOf(id, len(revs)+1, definition))
	}
	refreshLatest(item)
	resolvedMode := "new-revision"
	if overwrite {
		resolvedMode = "overwrite"
	}
	revs = revisions(item)
	return &Answer{Status: 200, Body: Item{"workflow": rec, "revision": revs[len(revs)-1], "latestAlias": aliases(item)["latest"], "mode": resolvedMode}}
}

// patch is PATCH {name, description, path, metadata, nrn}: the record only,
// no revision (observed against the API).
func (w *workflowFake) patch(col *collection, item, body Item, instance string) *Answer {
	rec := record(item)
	changes := Item{}
	for _, k := range []string{"name", "description", "path", "metadata", "nrn"} {
		if value, ok := body[k]; ok && value != nil {
			changes[k] = value
		}
	}
	if len(changes) == 0 {
		return invalid("PATCH body must include at least one of: name, path, description, metadata", instance) // {description: null} alone (observed against the API)
	}
	for _, k := range []string{"name", "description", "path", "nrn"} {
		if value, ok := changes[k]; ok {
			if _, isString := value.(string); !isString {
				return invalid(k+" must be a string", instance)
			}
		}
	}
	if name, ok := changes["name"]; ok && name == "" {
		return codeAnswer("NAME_REQUIRED", "name is required")
	}
	if nrn, ok := changes["nrn"].(string); ok && !nrnPattern.MatchString(nrn) {
		return codeAnswer("NRN_INVALID", "nrn is not valid")
	}
	if path, ok := changes["path"].(string); ok {
		// Segments joined by one "/", a leading one, no trailing one ("/" stays
		// "/"); "" removes the path (observed against the API).
		var parts []string
		for _, part := range strings.Split(path, "/") {
			if part != "" {
				parts = append(parts, part)
			}
		}
		if path == "" {
			delete(changes, "path")
			delete(rec, "path")
		} else {
			changes["path"] = "/" + strings.Join(parts, "/")
		}
	}
	for k, v := range changes {
		rec[k] = v
	}
	return &Answer{Status: 200, Body: Item{"workflow": rec}}
}

// delete refuses while an alias other than latest is active (observed against
// the API; the type is alias-already-exists); inactive aliases go with it.
func (w *workflowFake) delete(s *Server, col *collection, item Item, instance string) *Answer {
	id := Str(record(item), "id")
	for _, name := range sortedAliases(item) {
		alias := aliases(item)[name]
		if alias["active"] == true {
			return problemAnswer(409, "alias-already-exists", "Conflict", fmt.Sprintf("cannot delete workflow '%s': alias '%s' is still active", id, name), instance)
		}
	}
	if w.engine.UnreleasedOnDelete {
		for _, name := range sortedAliases(item) {
			if holdsChannels(s, aliases(item)[name]) {
				return notReleased(id, name, instance)
			}
		}
	}
	delete(col.items, id)
	return &Answer{Status: http.StatusNoContent}
}

func notReleased(id, name, instance string) *Answer {
	return problemAnswer(409, "alias-not-released", "Alias not released", fmt.Sprintf("alias '%s' of workflow '%s' still holds unreleased trigger channels; deactivate it first", name, id), instance) // detail: fake's choice
}

func sortedAliases(item Item) []string {
	var names []string
	for name := range aliases(item) {
		if name != "latest" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// aliasRoute handles …/aliases[/<a>[/activate|/deactivate]]; rest is what
// follows "aliases".
func (w *workflowFake) aliasRoute(s *Server, item Item, method string, rest []string, body Item, instance string) *Answer {
	id := Str(record(item), "id")
	if len(rest) == 0 {
		if method != http.MethodPost {
			return nil
		}
		name, _ := body["name"].(string)
		n, ok := toInt(body["revision"])
		switch {
		case name == "" || !ok:
			return invalid("name and revision are required", instance)
		case name == "latest":
			return invalid("the alias 'latest' is maintained by the system", instance) // 400 (observed against the API); text: fake's choice
		case n < 1 || n > len(revisions(item)):
			return revisionNotFound(id, n, instance)
		}
		if _, exists := aliases(item)[name]; exists {
			return problemAnswer(409, "alias-already-exists", "Conflict", fmt.Sprintf("alias '%s' already exists on workflow '%s'", name, id), instance)
		}
		alias := Item{"workflowId": id, "name": name, "revision": n, "active": false, "updatedAt": WorkflowTimestamp, "updatedBy": "fake-user"}
		aliases(item)[name] = alias
		return &Answer{Status: http.StatusCreated, Body: alias}
	}
	name := rest[0]
	alias, exists := aliases(item)[name]
	if !exists {
		return aliasNotFound(id, name, instance)
	}
	if len(rest) == 2 {
		if method != http.MethodPost {
			return nil
		}
		if name == "latest" {
			return invalid("the alias 'latest' is maintained by the system", instance)
		}
		switch rest[1] {
		case "activate": // the route reads no body: any is ignored (observed against the API)
			return w.activate(s, item, alias, instance)
		case "deactivate":
			return w.deactivate(s, alias)
		}
		return nil
	}
	switch method {
	case http.MethodGet:
		return &Answer{Status: 200, Body: alias}
	case http.MethodPut:
		n, ok := toInt(body["revision"])
		switch {
		case name == "latest":
			return invalid("the alias 'latest' is maintained by the system", instance)
		case !ok:
			return invalid("revision is required", instance)
		case n < 1 || n > len(revisions(item)):
			return revisionNotFound(id, n, instance)
		}
		// On an active alias it moves the pointer without re-registering
		// triggers (observed against the API).
		alias["revision"] = n
		return &Answer{Status: 200, Body: alias}
	case http.MethodDelete:
		switch {
		case name == "latest":
			return invalid("the alias 'latest' is maintained by the system", instance)
		case alias["active"] == true && w.engine.RefuseActiveAliasDelete:
			// (read from the API's code) 409 alias-active
			return problemAnswer(409, "alias-active", "Alias active", fmt.Sprintf("Alias '%s' is active; deactivate it before deleting", name), instance)
		case alias["active"] != true && w.engine.UnreleasedOnDelete && holdsChannels(s, alias):
			return notReleased(id, name, instance)
		}
		// Without the refusal it deletes active or not, its triggers left registered (observed
		// against the API).
		delete(aliases(item), name)
		return &Answer{Status: http.StatusNoContent}
	}
	return nil
}

// webhookURL: <server>/workflows/webhooks/<id>-<slug of the revision
// definition's name>/<alias>/<token>; the token is deterministic.
func webhookURL(s *Server, item Item, revision Item, alias, trigger string) string {
	id := Str(record(item), "id")
	name, _ := revision["definition"].(Item)["name"].(string)
	slug := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
	sum := sha256.Sum256([]byte(id + "/" + alias + "/" + trigger))
	return fmt.Sprintf("%s/workflows/webhooks/%s-%s/%s/%s", s.URL(), id, slug, alias, hex.EncodeToString(sum[:])[:32])
}

func channelIDsOf(alias Item) []int {
	states, _ := alias["triggerStates"].(Item)
	var ids []int
	for _, triggerID := range sortedKeys(states) {
		state, _ := states[triggerID].(Item)
		pluginState, _ := state["pluginState"].(Item)
		if n, ok := toInt(pluginState["channelId"]); ok {
			ids = append(ids, n)
		}
	}
	return ids
}

func sortedKeys(m Item) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func channelExists(s *Server, id int) (Item, bool) {
	item, ok := s.collections[NotificationChannels].items[strconv.Itoa(id)]
	return item, ok
}

// holdsChannels: an alias still has a channel registered by its activation.
func holdsChannels(s *Server, alias Item) bool {
	for _, id := range channelIDsOf(alias) {
		if _, ok := channelExists(s, id); ok {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// activate registers the triggers of the alias's revision: a webhook URL for
// each, and an http channel for each checklist trigger with config.nrn
// (observed against the API). A trigger without config.nrn creates no channel
// and no error (read from the API's code). A failure is a 200 with
// failedTriggers and alias.active false; the engine's own rollback is not
// known, so the fake releases what the failed attempt created and keeps no
// states (fake's choice).
func (w *workflowFake) activate(s *Server, item, alias Item, instance string) *Answer {
	// Re-activation tears down the previous activation's channels first, so no
	// duplicates (what the engine does is the fake's choice; the fake also
	// releases leftovers of a deactivation that could not).
	for _, n := range channelIDsOf(alias) {
		delete(s.collections[NotificationChannels].items, strconv.Itoa(n))
	}
	delete(alias, "triggerStates")

	n, _ := toInt(alias["revision"])
	revision := revisions(item)[n-1]
	name := Str(alias, "name")
	states := Item{}
	var created []int
	var failure, failedID string
	triggers, _ := revision["definition"].(Item)["triggers"].([]any)
	for _, t := range triggers {
		trigger := t.(map[string]any)
		triggerID, _ := trigger["id"].(string)
		plugin, _ := trigger["pluginType"].(string)
		config, _ := trigger["config"].(map[string]any)
		hook := webhookURL(s, item, revision, name, triggerID)
		state := Item{"status": "live", "activatedAt": WorkflowTimestamp, "runtimeMetadata": Item{"webhookUrl": hook}}
		if nrn, _ := config["nrn"].(string); plugin == checklistTrigger && nrn != "" {
			if containsString(w.engine.DeniedChannelNRNs, nrn) {
				failure, failedID = fmt.Sprintf("Nullplatform API error (403): cannot create a notification channel at %s", nrn), triggerID // text: fake's choice
				break
			}
			channel := checklistChannel(config, hook)
			channel["nrn"] = nrn
			s.collections[NotificationChannels].store(channel)
			created = append(created, numericID(channel))
			state["pluginState"] = Item{"channelId": channel["id"]}
		}
		states[triggerID] = state
	}
	// Every trigger of the revision is reported, each {id, pluginType, status,
	// error}; on a failure all of them failed, the one that caused it with its
	// error, the others rolled back (read from the API's code).
	var activated, failed, results []any
	for _, t := range triggers {
		trigger := t.(map[string]any)
		result := Item{"id": trigger["id"], "pluginType": trigger["pluginType"], "status": "active"}
		if failure != "" {
			result["status"], result["error"] = "failed", "rolled back: "+failure
			if trigger["id"] == failedID {
				result["error"] = failure
			}
			failed = append(failed, result)
		} else {
			activated = append(activated, trigger["id"])
		}
		results = append(results, result)
	}
	if failure != "" {
		// The alias is left inactive, and what the failed attempt created is
		// released (fake's choice: the engine's own rollback is not known).
		for _, c := range created {
			delete(s.collections[NotificationChannels].items, strconv.Itoa(c))
		}
		alias["active"] = false
		body := Item{"alias": alias, "activatedTriggers": []any{}, "failedTriggers": failed, "triggerResults": results}
		if w.engine.ReportActivationError {
			body["activationError"] = failure // the error of the trigger that failed (read from the API's code)
		}
		return &Answer{Status: 200, Body: body}
	}
	alias["active"] = true
	if len(states) > 0 {
		alias["triggerStates"] = states
	}
	if activated == nil {
		activated, results = []any{}, []any{}
	}
	return &Answer{Status: 200, Body: Item{"alias": alias, "activatedTriggers": activated, "failedTriggers": []any{}, "triggerResults": results}}
}

// checklistChannel is the channel an np-checklist-trigger step registers
// (observed against the API). config.channelActions, when set and not empty,
// overrides config.action; several actions filter with $in (fake's choice).
func checklistChannel(config map[string]any, hook string) Item {
	pathPrefix, _ := config["pathPrefix"].(string)
	kind, _ := config["kind"].(string)
	actions := []any{}
	if list, ok := config["channelActions"].([]any); ok && len(list) > 0 {
		actions = list
	} else if action, _ := config["action"].(string); action != "" {
		actions = []any{action}
	} else {
		actions = []any{defaultChecklist}
	}
	actionFilter := Item{"$eq": actions[0]}
	if len(actions) > 1 {
		actionFilter = Item{"$in": actions}
	}
	return Item{
		"type": "http", "source": []any{"checklist"}, "status": "active",
		"description":   fmt.Sprintf("Workflow checklist trigger: %s [kind=%s]", pathPrefix, kind),
		"filters":       Item{"$and": []any{Item{"action": actionFilter}, Item{"kind": Item{"$eq": kind}}}},
		"configuration": Item{"url": hook},
	}
}

// deactivate writes active false before tearing down, deletes the channels it
// can, and leaves triggerStates as they were (observed against the API). A
// teardown that failed is reported with deactivateError only when the engine
// reports it (read from the API's code).
func (w *workflowFake) deactivate(s *Server, alias Item) *Answer {
	alias["active"] = false
	states, _ := alias["triggerStates"].(Item)
	released := []any{}
	var failures []string
	for _, triggerID := range sortedKeys(states) {
		state, _ := states[triggerID].(Item)
		pluginState, _ := state["pluginState"].(Item)
		if n, ok := toInt(pluginState["channelId"]); ok {
			if channel, held := channelExists(s, n); held {
				nrn := Str(channel, "nrn")
				if containsString(w.engine.UnreleasableChannelNRNs, nrn) {
					if w.engine.ReportReleaseFailures {
						failures = append(failures, fmt.Sprintf("trigger '%s': cannot delete channel %d at %s", triggerID, n, nrn)) // text: fake's choice
						continue
					}
					continue // silently kept
				}
				delete(s.collections[NotificationChannels].items, strconv.Itoa(n))
			}
		}
		released = append(released, triggerID)
	}
	body := Item{"alias": alias, "deactivatedTriggers": released}
	if len(failures) > 0 {
		body["deactivateError"] = strings.Join(failures, "; ")
	}
	return &Answer{Status: 200, Body: body}
}

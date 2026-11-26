package fakeplatform

// Application types as the API keeps them, and what applications and
// templates do with one:
//
//   - a type gets its slug from its name on create and keeps it; it is created
//     active and visible at its own NRN unless the caller says otherwise;
//   - no two types of an organization share a name, and an inactive type
//     still holds its own;
//   - the schema is kept and answered with its keys in the order they were
//     sent, except that keys that read as integers come first, in ascending
//     order, as the API's JavaScript objects keep them;
//   - a change of the name, the description or the schema cuts a new
//     snapshot; a schema that differs from the stored one only in whitespace
//     is no change, and one that differs in key order is one;
//   - a deleted type answers 404 with its code, gives its name back, and is
//     refused while an application that is not deleted, or any template,
//     still points at it;
//   - a new application of a type takes a type only when it is active, starts
//     on the type's last snapshot, and is `pending_hook` until the type's
//     workflow answers its creation hook, which takes it more than one read.
//     Until then every edit of it is refused. When no hook is configured for
//     the creation of applications, it is stored `failed`, with a message;
//   - a new application with no type is `pending`, and the built-in workflow
//     makes it `creating`, then `active`, one read at a time;
//   - an application's status moves only along the API's transitions, and a
//     DELETE is refused unless it is `deleting`. A `deleting` one is left
//     `deleted` by its workflow, and keeps its name: no new application of
//     the namespace can take it;
//   - the attributes an application is sent are a JSON Merge Patch (RFC 7396)
//     of the ones it has: a null removes a member, an object is merged, any
//     other value replaces. They are checked against the schema of the
//     snapshot the application is on, or is moving to. An application with
//     no type has none, and an empty object sent to it is none;
//   - the ids of a type and of a snapshot are read in lower case;
//   - a template is edited member by member, a null or empty one meaning
//     "leave it", except its type, which a null clears.
//
// Refusals carry the API's whole error body: the error's static description
// as `message`, the specific text as `detail` only for the errors the API
// makes public, and a UUID string as `id`.
//
// Simplifications: the check of the attributes knows `required` only; the
// creation hook is looked for anywhere, not at the application's NRN; a type's
// visibility at the application's namespace is not checked; slugs are not
// numbered; a template's type is not checked against its application's.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/jsonorder"
)

// EntityHookAction is the collection of the platform's entity hook actions.
// An application of a type is created by its type's workflow through the hook
// of `application:create`; a test declares that hook by seeding it.
const EntityHookAction = "entity_hook/action"

// errorID stands for the UUID the API gives each error it can answer.
const errorID = "5f3c1e2a-9b7d-4c1e-8a2f-6d4b3c2a1e0f"

// apiError is the API's error body for one of its business errors.
func apiError(status int, code, message string, detail any) *Refusal {
	return &Refusal{Status: status, Body: Item{
		"type":    "urn:nullplatform:error:" + strings.ToLower(strings.ReplaceAll(code, "_", "-")),
		"title":   message,
		"status":  status,
		"code":    code,
		"detail":  detail,
		"message": message,
		"id":      errorID,
	}}
}

// The API's application statuses, and where each one may go: for an
// application with no type, and for one of a type.
var (
	untypedTransitions = map[string][]string{
		"pending_hook": {"pending", "failed"},
		"pending":      {"creating", "updating"},
		"creating":     {"active", "failed"},
		"updating":     {"active", "failed"},
		"active":       {"pending", "deleting", "failed"},
		"inactive":     {"pending", "deleting"},
		"failed":       {"pending", "deleting"},
		"deleting":     {"deleted", "failed"},
	}
	typedTransitions = map[string][]string{
		"pending_hook": {"active", "failed"},
		"active":       {"deleting", "failed"},
		"inactive":     {"deleting"},
		"failed":       {"deleting"},
		"deleting":     {"deleted", "failed"},
	}
)

// RegisterApplicationTypes mounts /application_type, /application, /template
// and /entity_hook/action.
func RegisterApplicationTypes(s *Server) {
	// Every snapshot cut: the type it was cut of, and the schema it holds,
	// which stays as it was when the type's changes.
	type snapshot struct {
		of     Item
		schema json.RawMessage
	}
	snapshots := map[string]snapshot{}
	cut := func(item Item) {
		id := fmt.Sprintf("snapshot-%d", len(snapshots)+1)
		item["last_snapshot_id"] = id
		snapshots[id] = snapshot{of: item, schema: item["schema"].(json.RawMessage)}
	}
	// Whether a snapshot was cut of a type.
	snapshotOf := func(typeID, snapshotID string) bool {
		cut, held := snapshots[snapshotID]
		return held && Str(cut.of, "id") == typeID
	}
	// The attributes refused by a snapshot's schema, as the API says it.
	refusedBy := func(snapshotID string, attributes Item) *Refusal {
		var typeSchema struct {
			Required []string `json:"required"`
		}
		_ = json.Unmarshal(snapshots[snapshotID].schema, &typeSchema)
		var failures []string
		for _, name := range typeSchema.Required {
			if _, held := attributes[name]; !held {
				failures = append(failures, fmt.Sprintf("data must have required property '%s'", name))
			}
		}
		if len(failures) == 0 {
			return nil
		}
		return apiError(http.StatusBadRequest, "APPLICATION.INVALID_ATTRIBUTES", "Invalid attributes: "+strings.Join(failures, ", "), nil)
	}

	deleted := map[string]bool{}
	s.Register("application_type", "type", Hooks{
		OnCreate: func(s *Server, item Item) *Refusal {
			if refusal := nameTaken(s, Str(item, "name"), item); refusal != nil {
				return refusal
			}
			item["slug"] = DeriveSlug(Str(item, "name"))
			if Str(item, "status") == "" {
				item["status"] = "active"
			}
			if _, set := item["visible_to"]; !set {
				item["visible_to"] = []any{item["nrn"]}
			}
			if _, set := item["description"]; !set {
				item["description"] = nil
			}
			item["schema"] = inJavaScriptKeyOrder(s.sentRaw("schema"))
			cut(item)
			return nil
		},
		OnPatch: func(s *Server, existing, patch Item) *Refusal {
			if name, renamed := patch["name"].(string); renamed {
				if refusal := nameTaken(s, name, existing); refusal != nil {
					return refusal
				}
			}
			changes := false
			for _, member := range []string{"name", "description"} {
				if value, sent := patch[member]; sent && value != existing[member] {
					changes = true
				}
			}
			if _, sent := patch["schema"]; sent {
				schema := inJavaScriptKeyOrder(s.sentRaw("schema"))
				if !sameCompact(existing["schema"].(json.RawMessage), schema) {
					changes = true
				}
				patch["schema"] = schema
			}
			delete(patch, "slug")
			delete(patch, "nrn")
			for key, value := range patch {
				existing[key] = value
			}
			if changes {
				cut(existing)
			}
			return nil
		},
		// A deleted type answers 404 with the API's code; an id the API never
		// had answers like one the caller may not see.
		OnMissing: func(_ *Server, id string) *Refusal {
			if deleted[id] {
				return apiError(http.StatusNotFound, "APPLICATION_TYPE.NOT_FOUND", "The application type was deleted", nil)
			}
			return &Refusal{Status: http.StatusForbidden, Message: "Forbidden"}
		},
		// A deleted type gives its name back. Refused while something still
		// points at it.
		OnDelete: func(s *Server, item Item) *Refusal {
			if using := pointingAt(s, "application", item); using > 0 {
				return stillUsed("APPLICATIONS_EXIST", "while applications of it exist",
					fmt.Sprintf("%s of this type still %s", counted(using, "application"), plural(using, "exists", "exist")))
			}
			if using := pointingAt(s, "template", item); using > 0 {
				return stillUsed("TEMPLATES_EXIST", "while templates belong to it",
					fmt.Sprintf("%s still %s to it", counted(using, "template"), plural(using, "belongs", "belong")))
			}
			deleted[Str(item, "id")] = true
			return nil
		},
	})

	s.Register(EntityHookAction, "hook", Hooks{})

	// How many times each application was read since its status last changed:
	// its workflow moves it on one read at a time. (The engine's Gets is keyed
	// by bare id, and an application and a template can share one.)
	reads := map[string]int{}
	setStatus := func(item Item, status string) {
		item["status"] = status
		reads[fmt.Sprint(item["id"])] = 0
	}

	s.RegisterWith("application", "app", Options{NumericIDs: true, PatchStatus: http.StatusNoContent}, Hooks{
		OnCreate: func(s *Server, item Item) *Refusal {
			if refusal := nameHeld(s, item, Str(item, "name")); refusal != nil {
				return refusal
			}
			attributes, _ := item["attributes"].(Item)
			typeID := strings.ToLower(Str(item, "application_type_id"))
			item["status"] = "pending"
			if typeID == "" {
				if len(attributes) > 0 {
					return apiError(http.StatusBadRequest, "APPLICATION.INVALID_ATTRIBUTES", "The attributes do not pass the schema of the application's type",
						"attributes belong to an application type, and this application has none")
				}
			} else {
				item["application_type_id"] = typeID
				applicationType, held := s.collections["application_type"].items[typeID]
				if !held || Str(applicationType, "status") != "active" {
					return apiError(http.StatusBadRequest, "APPLICATION.INVALID_APPLICATION_TYPE", "The application type cannot be used for this application",
						fmt.Sprintf("The application type %s cannot be used in namespace %v", typeID, item["namespace_id"]))
				}
				snapshotID := Str(applicationType, "last_snapshot_id")
				if refusal := refusedBy(snapshotID, mergePatch(Item{}, attributes)); refusal != nil {
					return refusal
				}
				item["application_type_snapshot_id"] = snapshotID
				// The type's workflow creates it, once the hook reaches it.
				item["status"] = "pending_hook"
				if len(s.collections[EntityHookAction].items) == 0 {
					item["status"] = "failed"
					item["messages"] = []any{Item{
						"level":   "ERROR",
						"message": "No hook could be delivered to create this application: its type has no hook action or no channel that takes it. Retry once there is one.",
					}}
				}
			}
			item["attributes"] = mergePatch(Item{}, attributes)
			item["desired_application_type_snapshot_id"] = nil
			// The API answers both for an application that has none.
			for _, member := range []string{"settings", "tags"} {
				if _, sent := item[member]; !sent {
					item[member] = Item{}
				}
			}
			return nil
		},
		// The application's workflow, one read at a time: the type's answers
		// the creation hook with the repository it made, the built-in one
		// creates one with no type, and either finishes a delete.
		OnGet: func(_ *Server, item Item) {
			id := fmt.Sprint(item["id"])
			reads[id]++
			// It takes a workflow a moment: the first read still finds it.
			if reads[id] < 2 {
				return
			}
			switch Str(item, "status") {
			case "pending_hook":
				setStatus(item, "active")
				if Str(item, "repository_url") == "" {
					item["repository_url"] = "https://github.com/acme/" + Str(item, "name")
				}
			case "pending":
				setStatus(item, "creating")
			case "creating":
				setStatus(item, "active")
			case "deleting":
				setStatus(item, "deleted")
			}
		},
		OnPatch: func(s *Server, existing, patch Item) *Refusal {
			// Nothing edits an application whose creation hook is not answered.
			if Str(existing, "status") == "pending_hook" {
				return apiError(http.StatusForbidden, "ENTITY_HOOKS.ENTITY_CREATION_HOOK_PENDING",
					"It's not possible to modify the entity as there's a pending creation hook for this entity for this user.", nil)
			}
			typed := Str(existing, "application_type_id") != ""
			sent, namesAttributes := patch["attributes"].(Item)
			if !typed && namesAttributes && len(sent) == 0 {
				// An empty object is none.
				namesAttributes = false
			}
			asked, namesDestination := patch["desired_application_type_snapshot_id"].(string)
			asked = strings.ToLower(asked)
			if !typed && (namesAttributes || namesDestination) {
				member, code, message := "desired_application_type_snapshot_id", "APPLICATION.INVALID_TYPE_SNAPSHOT", "The application cannot move to that snapshot of its type"
				if namesAttributes {
					member, code, message = "attributes", "APPLICATION.INVALID_ATTRIBUTES", "The attributes do not pass the schema of the application's type"
				}
				return apiError(http.StatusBadRequest, code, message, member+" belongs to an application type, and this application has none")
			}
			if namesDestination && !snapshotOf(Str(existing, "application_type_id"), asked) {
				return apiError(http.StatusBadRequest, "APPLICATION.INVALID_TYPE_SNAPSHOT", "The application cannot move to that snapshot of its type",
					asked+" is not a snapshot of the application's type")
			}
			if status, changes := patch["status"].(string); changes && status != Str(existing, "status") {
				transitions := untypedTransitions
				if typed {
					transitions = typedTransitions
				}
				allowed := false
				for _, next := range transitions[Str(existing, "status")] {
					allowed = allowed || next == status
				}
				if !allowed {
					return apiError(http.StatusBadRequest, "APPLICATION.INVALID_APP_STATUS", "The application status is invalid", nil)
				}
			}
			if name, renamed := patch["name"].(string); renamed && name != Str(existing, "name") {
				if refusal := nameHeld(s, existing, name); refusal != nil {
					return refusal
				}
			}

			// Naming the destination stores it; an edit of the application
			// moves it there, once the attributes pass the destination's
			// schema. Naming it alone is not one.
			destination := Str(existing, "desired_application_type_snapshot_id")
			if namesDestination {
				destination = asked
			}
			others := 0
			for key := range patch {
				if key != "attributes" && key != "desired_application_type_snapshot_id" {
					others++
				}
			}
			moves := destination != "" && (namesAttributes || others > 0 || destination == Str(existing, "application_type_snapshot_id"))
			attributes, _ := existing["attributes"].(Item)
			if namesAttributes {
				attributes = mergePatch(existing["attributes"], sent)
			}
			if typed && (namesAttributes || moves) {
				held := Str(existing, "application_type_snapshot_id")
				if moves {
					held = destination
				}
				if refusal := refusedBy(held, attributes); refusal != nil {
					return refusal
				}
			}

			if namesAttributes {
				existing["attributes"] = attributes
			}
			delete(patch, "attributes")
			delete(patch, "desired_application_type_snapshot_id")
			if namesDestination {
				existing["desired_application_type_snapshot_id"] = asked
			}
			if moves {
				existing["application_type_snapshot_id"] = destination
				existing["desired_application_type_snapshot_id"] = nil
			}
			if status, changes := patch["status"].(string); changes && status != Str(existing, "status") {
				setStatus(existing, status)
				delete(patch, "status")
			}
			for key, value := range patch {
				existing[key] = value
			}
			return nil
		},
		// The API deletes only what was first moved to `deleting`.
		OnDelete: func(_ *Server, item Item) *Refusal {
			if Str(item, "status") != "deleting" {
				return apiError(http.StatusBadRequest, "APPLICATION.INVALID_APP_STATUS", "The application status is invalid", nil)
			}
			return nil
		},
	})

	s.RegisterWith("template", "template", Options{NumericIDs: true}, Hooks{
		OnCreate: func(_ *Server, item Item) *Refusal {
			if typeID, typed := item["application_type_id"].(string); typed {
				item["application_type_id"] = strings.ToLower(typeID)
			}
			return nil
		},
		// A member sent null or empty is left as it is; only the type of a
		// template takes a null, which is how a template leaves its type.
		OnPatch: func(_ *Server, existing, patch Item) *Refusal {
			for key, value := range patch {
				if (value == nil || value == "") && key != "application_type_id" {
					continue
				}
				if typeID, typed := value.(string); typed && key == "application_type_id" {
					value = strings.ToLower(typeID)
				}
				existing[key] = value
			}
			return nil
		},
	})
}

// nameHeld refuses a name another application of the namespace holds,
// whatever its status: a deleted application keeps its name.
func nameHeld(s *Server, asking Item, name string) *Refusal {
	for _, other := range s.collections["application"].items {
		if fmt.Sprint(other["namespace_id"]) != fmt.Sprint(asking["namespace_id"]) || Str(other, "name") != name {
			continue
		}
		if asking["id"] != nil && fmt.Sprint(other["id"]) == fmt.Sprint(asking["id"]) {
			continue
		}
		return apiError(http.StatusBadRequest, "APPLICATION.DUPLICATE_APPLICATION", "The application already exists",
			fmt.Sprintf("Application %s with id: %v is already using name %s", name, other["id"], name))
	}
	return nil
}

// pointingAt counts the items of a collection whose application_type_id is
// this type's id; an application that was deleted no longer counts.
func pointingAt(s *Server, collection string, applicationType Item) int {
	count := 0
	for _, item := range s.collections[collection].items {
		if Str(item, "application_type_id") == Str(applicationType, "id") && Str(item, "status") != "deleted" {
			count++
		}
	}
	return count
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

func counted(count int, noun string) string {
	return fmt.Sprintf("%d %s", count, plural(count, noun, noun+"s"))
}

// stillUsed is the API's 409 for a type that cannot be deleted yet.
func stillUsed(code, why, detail string) *Refusal {
	return apiError(http.StatusConflict, "APPLICATION_TYPE."+code, "The application type cannot be deleted "+why,
		"The application type cannot be deleted: "+detail)
}

// nameTaken refuses a name another type of the same organization holds,
// whatever that type's status. (The API compares the organizations each type
// reaches through visible_to as well; the fake keeps to the owner's.)
func nameTaken(s *Server, name string, asking Item) *Refusal {
	organization := func(item Item) string {
		owner, _, _ := strings.Cut(Str(item, "nrn"), ":")
		return owner
	}
	for _, other := range s.collections["application_type"].items {
		if Str(other, "name") == name && Str(other, "id") != Str(asking, "id") && organization(other) == organization(asking) {
			return Refuse("There is already an application type named %q where this one would be visible", name)
		}
	}
	return nil
}

// sentRaw is a member of the body being answered, byte for byte as the client
// sent it: decoding it into an Item would lose the order of its keys.
func (s *Server) sentRaw(member string) json.RawMessage {
	var body map[string]json.RawMessage
	_ = json.Unmarshal([]byte(s.requests[len(s.requests)-1].Body), &body)
	return body[member]
}

func sameCompact(a, b json.RawMessage) bool {
	var compactA, compactB bytes.Buffer
	_ = json.Compact(&compactA, a)
	_ = json.Compact(&compactB, b)
	return compactA.String() == compactB.String()
}

// inJavaScriptKeyOrder is a document as the API answers it once it has been
// a JavaScript object (see jsonorder), compact.
func inJavaScriptKeyOrder(document json.RawMessage) json.RawMessage {
	ordered, err := jsonorder.JavaScript(document)
	if err != nil {
		return document
	}
	return ordered
}

// mergePatch applies `patch` to `target` as a JSON Merge Patch (RFC 7396).
func mergePatch(target any, patch Item) Item {
	merged := Item{}
	if held, isObject := target.(Item); isObject {
		for name, value := range held {
			merged[name] = value
		}
	}
	for name, value := range patch {
		switch value := value.(type) {
		case nil:
			delete(merged, name)
		case Item:
			merged[name] = mergePatch(merged[name], value)
		default:
			merged[name] = value
		}
	}
	return merged
}

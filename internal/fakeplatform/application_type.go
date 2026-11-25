package fakeplatform

// Application types as the API keeps them, and what applications and
// templates do with one:
//
//   - a type gets its slug from its name on create and keeps it; it is created
//     active and visible at its own NRN unless the caller says otherwise;
//   - no two types of an organization share a name, and an inactive type
//     still holds its own;
//   - the schema is kept and answered with its keys in the order they were
//     sent, and a schema that differs from the stored one in anything but
//     whitespace — key order included — cuts a new snapshot;
//   - a type is never deleted: the API has no such route;
//   - a new application takes a type only when it is active, starts on the
//     type's last snapshot, and the attributes it is sent are a JSON Merge
//     Patch (RFC 7396) of the ones it has: a null removes a member, an object
//     is merged, any other value replaces; an application with no type has
//     none;
//   - a template is edited member by member, a null or empty one meaning
//     "leave it", except its type, which a null clears.
//
// The API also checks attributes against the snapshot's schema; the fake does
// not, so no test here speaks for that message.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// RegisterApplicationTypes mounts /application_type, /application and
// /template.
func RegisterApplicationTypes(s *Server) {
	snapshots := 0
	cut := func(item Item) {
		snapshots++
		item["last_snapshot_id"] = fmt.Sprintf("snapshot-%d", snapshots)
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
			item["schema"] = s.sentRaw("schema")
			cut(item)
			return nil
		},
		OnPatch: func(s *Server, existing, patch Item) *Refusal {
			if name, renamed := patch["name"].(string); renamed {
				if refusal := nameTaken(s, name, existing); refusal != nil {
					return refusal
				}
			}
			if _, sent := patch["schema"]; sent {
				schema := s.sentRaw("schema")
				if !sameCompact(existing["schema"].(json.RawMessage), schema) {
					cut(existing)
				}
				patch["schema"] = schema
			}
			delete(patch, "slug")
			delete(patch, "nrn")
			for key, value := range patch {
				existing[key] = value
			}
			return nil
		},
		// A deleted type answers 404 with the API's code; an id the API never
		// had answers like one the caller may not see.
		OnMissing: func(_ *Server, id string) *Refusal {
			if deleted[id] {
				return &Refusal{Status: http.StatusNotFound, Body: Item{
					"code":    "APPLICATION_TYPE.NOT_FOUND",
					"message": "The application type was deleted",
				}}
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

	s.RegisterWith("application", "app", Options{NumericIDs: true}, Hooks{
		OnCreate: func(s *Server, item Item) *Refusal {
			attributes, _ := item["attributes"].(Item)
			typeID := Str(item, "application_type_id")
			if typeID == "" {
				if len(attributes) > 0 {
					return Refuse("attributes belong to an application type, and this application has none")
				}
			} else {
				applicationType, held := s.collections["application_type"].items[typeID]
				if !held || Str(applicationType, "status") != "active" {
					return Refuse("The application type %s cannot be used in namespace %v", typeID, item["namespace_id"])
				}
				item["application_type_snapshot_id"] = applicationType["last_snapshot_id"]
			}
			item["attributes"] = mergePatch(Item{}, attributes)
			item["status"] = "active"
			return nil
		},
		OnPatch: func(_ *Server, existing, patch Item) *Refusal {
			if sent, ok := patch["attributes"].(Item); ok {
				if Str(existing, "application_type_id") == "" {
					return Refuse("attributes belongs to an application type, and this application has none")
				}
				existing["attributes"] = mergePatch(existing["attributes"], sent)
				delete(patch, "attributes")
			}
			for key, value := range patch {
				existing[key] = value
			}
			return nil
		},
	})

	s.RegisterWith("template", "template", Options{NumericIDs: true}, Hooks{
		// A member sent null or empty is left as it is; only the type of a
		// template takes a null, which is how a template leaves its type.
		OnPatch: func(_ *Server, existing, patch Item) *Refusal {
			for key, value := range patch {
				if (value == nil || value == "") && key != "application_type_id" {
					continue
				}
				existing[key] = value
			}
			return nil
		},
	})
}

// pointingAt counts the items of a collection whose application_type_id is
// this type's id.
func pointingAt(s *Server, collection string, applicationType Item) int {
	count := 0
	for _, item := range s.collections[collection].items {
		if Str(item, "application_type_id") == Str(applicationType, "id") {
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
	return &Refusal{Status: http.StatusConflict, Body: Item{
		"code":    "APPLICATION_TYPE." + code,
		"message": "The application type cannot be deleted " + why,
		"detail":  "The application type cannot be deleted: " + detail,
	}}
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

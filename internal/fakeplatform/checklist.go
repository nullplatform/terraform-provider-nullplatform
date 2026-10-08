package fakeplatform

// The checklist specification contract as behavior hooks, with the API's
// messages literal; a rule not marked as observed against the API was read
// from its code. Not emulated, because the provider never sends it: the
// show_descendants, status and name:contains filters, sort, ?force=true on
// DELETE, moving actions to another version, the 400 for a list without nrn,
// the validator's rules past items.required and the conflict of two
// concurrent PATCHes on one lineage.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const ChecklistSpecification = "approval/checklist/specification"

// problem is an error body as the API answers it (observed against the API):
// RFC 7807, message == title, detail a string or null, id 0.
func problem(status int, title string, detail any) *Refusal {
	return &Refusal{Status: status, Message: title, Body: Item{
		"type": "about:blank", "title": title, "status": status, "detail": detail, "message": title, "id": 0,
	}}
}

// RegisterChecklist mounts /approval/checklist/specification. A version is
// in use while an /approval/action item points at it.
func RegisterChecklist(s *Server) {
	s.RegisterWith(ChecklistSpecification, "spec", Options{
		CreateStatus: http.StatusCreated, // observed against the API
		KeepDeleted:  true,
		IDFormat:     "spec_%016d", // spec_ and 16 characters
	}, Hooks{
		OnCreate: func(s *Server, item Item) *Refusal {
			if refusal := validateDefinition(item); refusal != nil {
				return refusal
			}
			// (nrn, name, version) is unique and every lineage has a version 1, so
			// a used (nrn, name) is refused, a deleted one too (observed against
			// the API).
			for _, existing := range s.collections[ChecklistSpecification].items {
				if Str(existing, "nrn") == Str(item, "nrn") && Str(existing, "name") == Str(item, "name") {
					return problem(http.StatusBadRequest, "A checklist specification with the same nrn/name/version already exists", nil)
				}
			}
			// Every read orders the definition; this orders the POST's answer
			// (observed against the API).
			item["definition"] = jsonb(item["definition"])
			if Str(item, "description") == "" {
				item["description"] = nil
			}
			item["status"], item["version"] = "active", 1
			return nil
		},
		OnPatchNew: func(s *Server, existing, patch Item) (Item, *Refusal) {
			// A PATCH inserts a new row, version max+1 of (nrn, name); what it leaves
			// out is inherited, nrn always; the old row stays as it was (observed
			// against the API).
			next := Item{"nrn": existing["nrn"], "status": "active"}
			for _, key := range []string{"name", "description", "definition"} {
				next[key] = existing[key]
				if value, ok := patch[key]; ok {
					next[key] = value
				}
			}
			// A PATCH's 422 is a create's, not observed against the API.
			if refusal := validateDefinition(next); refusal != nil {
				return nil, refusal
			}
			next["definition"] = jsonb(next["definition"]) // the PATCH's answer
			version := 0
			for _, row := range lineage(s, next) {
				version = max(version, row["version"].(int))
			}
			next["version"] = version + 1
			return next, nil
		},
		OnGet: func(s *Server, item Item) {
			// The detail read always carries the actions on this version, the list
			// cut at 50 and the count whole; include_versions adds the lineage, each
			// version with its own count (observed against the API). The
			// definition's key order is the read's. The provider always sends
			// include_versions=true: "1" is kept for fidelity to the API.
			actions := referencing(s, Str(item, "id"))
			item["associated_actions"], item["associated_actions_count"] = actions[:min(len(actions), 50)], len(actions)
			item["definition"] = jsonb(item["definition"])
			delete(item, "versions")
			if flag := s.Query().Get("include_versions"); flag == "true" || flag == "1" {
				var versions []Item
				for _, row := range lineage(s, item) {
					versions = append(versions, summary(s, row))
				}
				item["versions"] = versions
			}
		},
		OnDelete: func(s *Server, item Item) *Refusal {
			// Soft, per version; refused while an action is on this version, with no
			// ids in the body, and allowed for an older version of a lineage in use
			// (observed against the API).
			if actions := referencing(s, Str(item, "id")); len(actions) > 0 {
				return problem(http.StatusConflict, "Checklist specification is in use by one or more approval actions",
					"Checklist specification "+Str(item, "id")+" is referenced by "+strconv.Itoa(len(actions))+" approval action(s)")
			}
			item["status"] = "deleted"
			return nil
		},
		OnMissing: func(_ *Server, _ string) *Refusal {
			// With a null detail, not observed against the API.
			return problem(http.StatusNotFound, "Checklist specification not found", nil)
		},
		OnList: listChecklistSpecifications,
	})
}

// validateDefinition is the API's first rule for a definition: items must be a
// non-empty array. Its detail is literal, observed against the API.
func validateDefinition(item Item) *Refusal {
	definition, ok := item["definition"].(Item)
	if !ok {
		definition, _ = item["definition"].(jsonbObject)
	}
	if items, _ := definition["items"].([]any); len(items) > 0 {
		return nil
	}
	return problem(http.StatusUnprocessableEntity, "Checklist specification validation failed",
		`Checklist specification validation failed: [{"rule":"items.required","path":"definition.items","message":"items must be a non-empty array"}]`)
}

// jsonbObject is an object as the API answers one it stored: keys shorter
// first, then bytewise, never as written (observed against the API:
// execution_trigger sent first comes back after items; an item's keys come
// back id, type, title, behavior, severity, description). Integer keys come
// first, ascending, and no HTML is escaped.
type jsonbObject map[string]any

// jsonb turns every object inside v into a jsonbObject.
func jsonb(v any) any {
	switch value := v.(type) {
	case map[string]any:
		out := jsonbObject{}
		for key, inner := range value {
			out[key] = jsonb(inner)
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, inner := range value {
			out[i] = jsonb(inner)
		}
		return out
	}
	return v
}

func (o jsonbObject) MarshalJSON() ([]byte, error) {
	keys := make([]string, 0, len(o))
	for key := range o {
		keys = append(keys, key)
	}
	// Integer keys first; among them, shorter then bytewise already is the
	// numeric order, since their text is canonical.
	sort.Slice(keys, func(i, j int) bool {
		if aIndex, bIndex := arrayIndex(keys[i]), arrayIndex(keys[j]); aIndex != bIndex {
			return aIndex
		}
		return len(keys[i]) < len(keys[j]) || (len(keys[i]) == len(keys[j]) && keys[i] < keys[j])
	})
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	out.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := enc.Encode(key); err != nil {
			return nil, err
		}
		out.Truncate(out.Len() - 1)
		out.WriteByte(':')
		if err := enc.Encode(o[key]); err != nil {
			return nil, err
		}
		out.Truncate(out.Len() - 1)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// arrayIndex reports whether the API orders key first: a canonical integer
// below 2^32-1.
func arrayIndex(key string) bool {
	n, err := strconv.ParseUint(key, 10, 32)
	return err == nil && n < 1<<32-1 && strconv.FormatUint(n, 10) == key
}

// lineage is every version of item's (nrn, name), ascending.
func lineage(s *Server, item Item) []Item {
	var rows []Item
	for _, row := range s.collections[ChecklistSpecification].items {
		if Str(row, "nrn") == Str(item, "nrn") && Str(row, "name") == Str(item, "name") {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["version"].(int) < rows[j]["version"].(int) })
	return rows
}

// referencing is the live actions linked to a version, by id.
func referencing(s *Server, specID string) []Item {
	actions := []Item{}
	if col, ok := s.collections[ApprovalAction]; ok {
		for _, action := range col.items {
			if Str(action, "checklist_specification_id") == specID && Str(action, "status") != "deleted" {
				actions = append(actions, Item{"id": action["id"], "nrn": action["nrn"], "entity": action["entity"],
					"action": action["action"], "dimensions": action["dimensions"], "status": action["status"]})
			}
		}
	}
	// Ordered by id, as integers.
	sort.Slice(actions, func(i, j int) bool {
		a, _ := actions[i]["id"].(int)
		b, _ := actions[j]["id"].(int)
		return a < b
	})
	return actions
}

// summary is a version as a list or a lineage shows it: its count, no lists.
func summary(s *Server, row Item) Item {
	out := Item{}
	for key, value := range row {
		if key != "versions" && key != "associated_actions" {
			out[key] = value
		}
	}
	out["definition"] = jsonb(row["definition"])
	out["associated_actions_count"] = len(referencing(s, Str(row, "id")))
	return out
}

// listChecklistSpecifications is GET /approval/checklist/specification: the
// nrn and its ancestors, or only it with no-merge; deleted versions hidden;
// latest_only keeps the newest live version of each lineage; offset and limit
// (default 50, at most 200) page it (observed against the API). The provider
// always sends no-merge, latest_only and a limit of 200: the ancestors, the
// list without latest_only and the cap are kept for fidelity to the API.
func listChecklistSpecifications(s *Server) (any, *Refusal) {
	query := s.Query()
	nrn, noMerge := query.Get("nrn"), query.Get("no-merge") == "true"
	newest := map[string]Item{}
	var rows []Item
	for _, row := range s.collections[ChecklistSpecification].items {
		rowNrn := Str(row, "nrn")
		if Str(row, "status") == "deleted" || (rowNrn != nrn && (noMerge || !strings.HasPrefix(nrn, rowNrn+":"))) {
			continue
		}
		key := rowNrn + "::" + Str(row, "name")
		if query.Get("latest_only") != "true" {
			key = Str(row, "id")
		}
		if kept, ok := newest[key]; !ok || row["version"].(int) > kept["version"].(int) {
			newest[key] = row
		}
	}
	for _, row := range newest {
		rows = append(rows, summary(s, row))
	}
	sort.Slice(rows, func(i, j int) bool { return Str(rows[i], "id") < Str(rows[j], "id") })
	offset, _ := strconv.Atoi(query.Get("offset"))
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 200)
	page := rows[min(offset, len(rows)):min(offset+limit, len(rows))]
	return Item{"results": page, "total": len(rows), "offset": offset, "limit": limit}, nil
}

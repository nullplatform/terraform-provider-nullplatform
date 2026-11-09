package fakeplatform

// Service and link specifications as the API creates them: a specification
// gets a slug on create — the one the caller gave, or one derived from the
// name (lowercase letters and digits joined by single dashes) — and that slug
// never changes afterwards, whatever a later write says. The document fields a
// create may omit default to empty objects, as the API's schema defaults them,
// and a specification has one snapshot from its create on.

import "strings"

// RegisterSpecifications mounts /service_specification and /link_specification.
func RegisterSpecifications(s *Server) {
	for name, prefix := range map[string]string{"service_specification": "sspec", "link_specification": "lspec"} {
		s.Register(name, prefix, Hooks{
			OnCreate:  func(_ *Server, item Item) *Refusal { specificationOnCreate(item); return nil },
			OnPatch:   func(_ *Server, existing, patch Item) *Refusal { keepSlug(existing, patch); return nil },
			OnSubList: specificationSubList,
		})
	}
}

func specificationOnCreate(item Item) {
	if Str(item, "slug") == "" {
		item["slug"] = DeriveSlug(Str(item, "name"))
	}
	for _, document := range []string{"dimensions", "attributes", "scopes"} {
		if _, set := item[document]; !set {
			item[document] = map[string]any{}
		}
	}
}

func keepSlug(existing, patch Item) {
	delete(patch, "slug")
	for key, value := range patch {
		existing[key] = value
	}
}

// What hangs off a specification: the snapshot minted at create, and the
// action specifications (none — the fake generates no default actions).
func specificationSubList(_ *Server, parent Item, sub string) (any, *Refusal) {
	switch sub {
	case "snapshots":
		return map[string]any{"results": []any{map[string]any{"id": Str(parent, "id") + "-snapshot-1", "sequence_number": 1}}}, nil
	case "action_specification":
		return map[string]any{"results": []any{}}, nil
	}
	return map[string]any{"results": []any{}}, nil
}

// DeriveSlug is the API's derivation, as far as a test needs it: lowercase,
// any run of other characters becomes one dash, no dash at either end.
func DeriveSlug(name string) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		alnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		switch {
		case alnum:
			out.WriteRune(r)
			dash = false
		case !dash && out.Len() > 0:
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(out.String(), "-")
}

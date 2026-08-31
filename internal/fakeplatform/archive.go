package fakeplatform

// The archive contract for services and links, as behavior hooks. Every rule
// here is a behavior any client can observe against the real API — including
// the refusal messages, which providers surface to operators verbatim.

import "strings"

// Chain is how a specification resolves a `PATCH {status}` archive/restore —
// the real API decides this from the specification's action set, so the fake
// keys it on specification_id too (RegisterArchive's chains map).
type Chain string

const (
	// ChainNone: no archive/unarchive actions — the direct flip.
	ChainNone Chain = "none"
	// ChainManaged: managed actions — the PATCH only starts an asynchronous
	// transition (`archiving`, and `updating` for the restore).
	ChainManaged Chain = "managed"
	// ChainManual: an archive action exists but is not managed — the status
	// shortcut is refused; the action is the only door.
	ChainManual Chain = "manual"
	// ChainApproval: managed behind an approval — the minted action parks in
	// pending_create and the instance stays put until someone decides.
	ChainApproval Chain = "approval"
	// ChainApprovalDenied: the approval is denied — the parked action
	// vanishes and the status never moves.
	ChainApprovalDenied Chain = "approval-denied"
)

const ArchivedAt = "2026-08-07T12:00:00.000Z"

// RegisterArchive mounts /service and /link with the archive contract.
// chains maps a specification_id to its chain; unlisted specs are ChainNone.
func RegisterArchive(s *Server, chains map[string]Chain) {
	chainOf := func(item Item) Chain {
		if chain, ok := chains[Str(item, "specification_id")]; ok {
			return chain
		}
		return ChainNone
	}

	s.Register("service", "svc", Hooks{
		OnCreate: func(s *Server, item Item) *Refusal {
			// The create-collision rule: a create matching an ARCHIVED twin is
			// refused with the aviso naming the twin and the way out.
			for _, existing := range s.collections["service"].items {
				if Str(existing, "status") == "archived" &&
					Str(existing, "name") == Str(item, "name") &&
					Str(existing, "specification_id") == Str(item, "specification_id") &&
					Str(existing, "entity_nrn") == Str(item, "entity_nrn") {
					return Refuse("An archived service (id %s) with the same specification, entity and "+
						"dimensions already exists - unarchive it, or request its deletion", Str(existing, "id"))
				}
			}
			return nil
		},
		OnGet:   func(s *Server, item Item) { progressService(s, item) },
		OnPatch: func(s *Server, item, patch Item) *Refusal { return patchService(s, item, patch, chainOf(item)) },
	})

	s.Register("link", "lnk", Hooks{
		OnCreate: func(_ *Server, item Item) *Refusal {
			item["status"] = "active"
			return nil
		},
		OnPatch: func(s *Server, item, patch Item) *Refusal { return patchLink(s, item, patch, chainOf(item)) },
	})
}

func progressService(s *Server, item Item) {
	id := Str(item, "id")
	switch {
	case Str(item, "status") == "archiving" && s.Gets(id) > 1:
		item["status"] = "archived"
		item["archived_at"] = ArchivedAt
	case Str(item, "status") == "updating" && s.Gets(id) > 1:
		item["status"] = "active"
		item["archived_at"] = ""
	default:
		actions, _ := item["actions_in_progress"].([]Item)
		if len(actions) > 0 && Str(actions[0], "status") == "pending_create" && s.Gets(id) > 2 {
			if strings.HasSuffix(Str(actions[0], "verdict"), "denied") {
				// Denial: the parked action vanishes; the status never moves.
				delete(item, "actions_in_progress")
				return
			}
			// Approval: the action starts running and the transition begins.
			actions[0]["status"] = "in_progress"
			item["status"] = "archiving"
			s.ResetGets(id)
		}
	}
}

func patchService(s *Server, item, patch Item, chain Chain) *Refusal {
	status := Str(item, "status")
	isArchive := Str(patch, "status") == "archived"
	isRestore := Str(patch, "status") == "active" && status == "archived"

	// An archive/restore request cannot carry attributes; providers must send
	// them separately, first.
	if (isArchive || isRestore) && patch["attributes"] != nil {
		return Refuse("An archive request cannot carry attributes. Apply attribute changes " +
			"separately, then PATCH the status on its own.")
	}

	switch {
	case isArchive:
		if chain == ChainManual {
			return Refuse("The specification defines an 'archive' action; use the 'archive' action " +
				"instead of PATCHing the status")
		}
		if status != "active" && status != "failed" && status != "cancelled" {
			return Refuse("Only active, cancelled or failed services can be archived")
		}
		// A service archives only once every one of its links has.
		for _, link := range s.collections["link"].items {
			if Str(link, "service_id") == Str(item, "id") && Str(link, "status") != "archived" {
				return Refuse("Service has non-archived links and cannot be archived; archive its links first")
			}
		}
		switch chain {
		case ChainApproval, ChainApprovalDenied:
			verdict := "approved"
			if chain == ChainApprovalDenied {
				verdict = "denied"
			}
			item["actions_in_progress"] = []Item{{
				"id": "act-1", "type": "archive", "status": "pending_create", "verdict": verdict,
			}}
			s.ResetGets(Str(item, "id"))
		case ChainManaged:
			item["status"] = "archiving"
			s.ResetGets(Str(item, "id"))
		default:
			item["status"] = "archived"
			item["archived_at"] = ArchivedAt
		}
	case isRestore:
		if chain == ChainManaged {
			item["status"] = "updating"
			s.ResetGets(Str(item, "id"))
		} else {
			item["status"] = "active"
			item["archived_at"] = ""
		}
	default:
		if Str(patch, "status") != "" {
			item["status"] = Str(patch, "status")
		}
	}
	if patch["attributes"] != nil {
		item["attributes"] = patch["attributes"]
	}
	return nil
}

func patchLink(s *Server, item, patch Item, _ Chain) *Refusal {
	switch {
	case Str(patch, "status") == "archived":
		item["status"] = "archived"
		item["archived_at"] = ArchivedAt
	case Str(patch, "status") == "active" && Str(item, "status") == "archived":
		// A link only comes back under a working parent.
		parentID := Str(item, "service_id")
		if parent, ok := s.collections["service"].items[parentID]; ok && Str(parent, "status") != "active" {
			return Refuse("Service %s must be active to unarchive its links", parentID)
		}
		item["status"] = "active"
		item["archived_at"] = ""
	case Str(patch, "status") != "":
		item["status"] = Str(patch, "status")
	}
	if patch["attributes"] != nil {
		item["attributes"] = patch["attributes"]
	}
	return nil
}

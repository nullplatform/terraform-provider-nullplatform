package fakeplatform

// The approval contract — actions, policies and the action↔policy
// association — as behavior hooks. Every rule is one the real API enforces,
// with its messages literal; a rule not marked as observed against the API was
// read from its code.

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
)

const (
	ApprovalAction = "approval/action"
	ApprovalPolicy = "approval/policy"
)

// approvalNotReadable is what GET /approval/action/<id> answers for an id that
// does not exist: the authorization check finds no entity and answers 401, in
// the API's problem shape, whose "status" is a number. Observed against the
// API for a missing id: 401, "status":401, "code":"unauthorized", this message.
var approvalNotReadable = Item{
	"type":    "about:blank",
	"title":   "You're not authorized to perform this operation.",
	"status":  http.StatusUnauthorized,
	"code":    "unauthorized",
	"detail":  nil,
	"message": "You're not authorized to perform this operation.",
}

// Approvals holds what the engine cannot: the association rows, live or
// archived. They are not fields of the action, so a GET never echoes them.
type Approvals struct {
	s            *Server
	associations map[string][]*association // by action id
	release      map[string]int            // refused links left before the live associations go
	unlink       map[string]int            // refused associations left before the link goes
}

type association struct {
	policyID string
	archived bool // set by a migration: the row stays, out of the live ones
}

// RegisterApproval mounts /approval/action, with its /policy associations,
// and /approval/policy.
func RegisterApproval(s *Server) *Approvals {
	a := &Approvals{s: s, associations: map[string][]*association{}, release: map[string]int{}, unlink: map[string]int{}}

	// POST answers 200 (the provider requires it); plain CRUD otherwise.
	s.RegisterWith(ApprovalPolicy, "policy", Options{NumericIDs: true}, Hooks{})

	s.RegisterWith(ApprovalAction, "action", Options{
		NumericIDs:  true,
		PatchStatus: http.StatusNoContent, // observed against the API
		KeepDeleted: true,
	}, Hooks{
		OnCreate: func(s *Server, item Item) *Refusal {
			// Absent dimensions are {} before anything else, so the provider's
			// request without them (omitempty) is compared too.
			if item["dimensions"] == nil {
				item["dimensions"] = Item{}
			}
			// An active action with the same nrn, entity, action and dimensions is
			// refused; a deleted one does not count.
			for _, existing := range s.collections[ApprovalAction].items {
				if Str(existing, "status") == "active" && Str(existing, "nrn") == Str(item, "nrn") &&
					Str(existing, "entity") == Str(item, "entity") && Str(existing, "action") == Str(item, "action") &&
					reflect.DeepEqual(existing["dimensions"], item["dimensions"]) {
					return Refuse("There is already an action for the given parameters")
				}
			}
			// Without on_policy_* the API puts `manual`.
			for _, key := range []string{"on_policy_success", "on_policy_fail"} {
				if Str(item, key) == "" {
					item[key] = "manual"
				}
			}
			// The fail path is validated on create too; absent, it is null. The
			// provider never sends an invalid one (its plan refuses it): kept for
			// fidelity to the API.
			if refusal := validateChecklistFail(item["on_checklist_fail"]); refusal != nil {
				return refusal
			}
			if _, ok := item["on_checklist_fail"]; !ok {
				item["on_checklist_fail"] = nil
			}
			item["status"] = "active"
			item["checklist_specification_id"] = nil
			item["checklist_template_id"] = nil
			return nil
		},
		OnGet: func(_ *Server, item Item) { item["policies"] = a.policiesOf(idString(item)) },
		OnPatch: func(_ *Server, item, patch Item) *Refusal {
			// A truthy on_policy_* outside its callbacks is refused; a falsy one is
			// ignored.
			callbacks := map[string][2]string{"on_policy_success": {"manual", "approve"}, "on_policy_fail": {"manual", "deny"}}
			for key, valid := range callbacks {
				if v := Str(patch, key); v != "" && v != valid[0] && v != valid[1] {
					return Refuse("Invalid policy callback. It must be one of the following: manual, approve, deny")
				}
			}
			// The PATCH maps only these three; entity, action and anything else are
			// ignored, and it still answers 204 (observed against the API).
			for key := range callbacks {
				if v := Str(patch, key); v != "" {
					item[key] = v
				}
			}
			if v, ok := patch["on_checklist_fail"]; ok {
				// Inert for the provider, whose plan refuses an invalid value: kept for
				// fidelity to the API.
				if refusal := validateChecklistFail(v); refusal != nil {
					return refusal
				}
				item["on_checklist_fail"] = v
			}
			return nil
		},
		OnDelete: func(_ *Server, item Item) *Refusal {
			// A soft delete — status `deleted`, still readable — that destroys the
			// live associations; archived rows stay.
			item["status"] = "deleted"
			a.dropLive(idString(item), "")
			return nil
		},
		OnMissing: func(_ *Server, id string) *Refusal {
			// A non-numeric id matches no route: the router's 404, not observed
			// against the API.
			if _, err := strconv.Atoi(id); err != nil {
				return &Refusal{Status: http.StatusNotFound, Body: Item{
					"message": "This is not a nullplatform.io entity or the HTTP verb is not supported.",
				}}
			}
			return &Refusal{Status: http.StatusUnauthorized, Body: approvalNotReadable}
		},
		OnSub: func(_ *Server, parent Item, method, sub, subID string, body Item) (int, *Refusal) {
			id := idString(parent)
			switch {
			case sub == "policy" && method == http.MethodPost && subID == "":
				// Refused while a specification is linked (observed against the API).
				if spec := Str(parent, "checklist_specification_id"); spec != "" {
					// Without UnlinkAfter the count stays at zero: the guard only keeps
					// it from going negative, which would change nothing.
					if a.unlink[id] > 0 {
						if a.unlink[id]--; a.unlink[id] == 0 {
							parent["checklist_specification_id"], parent["checklist_template_id"] = nil, nil
						}
					}
					return 0, problem(http.StatusConflict, "Approval action already has a checklist specification; cannot also associate policies",
						"Approval action "+id+" has checklist specification "+spec+"; cannot associate policies")
				}
				// A live association with the policy is refused; an archived one does
				// not count, so going back to the same policy works (observed against
				// the API, after an unlink: 201). The refusal's message is not
				// observed against the API.
				for _, assoc := range a.associations[id] {
					if !assoc.archived && assoc.policyID == Str(body, "policy_id") {
						return 0, problem(http.StatusBadRequest, "Policy is already associated with approval action", nil)
					}
				}
				// 201, without a body.
				a.associations[id] = append(a.associations[id], &association{policyID: Str(body, "policy_id")})
				return http.StatusCreated, nil
			case sub == "policy" && method == http.MethodDelete && subID != "":
				// Destroys the live row; for an archived one the 204 changes nothing
				// (observed against the API).
				a.dropLive(id, subID)
				return http.StatusNoContent, nil
			case sub == "checklist_specification" && method == http.MethodPost && subID == "":
				return a.link(parent, Str(body, "checklist_specification_id"))
			case sub == "checklist_specification" && method == http.MethodDelete && subID == "":
				// 204, also when nothing is linked; the fail path stays as it is
				// (observed against the API). Under UnlinkAfter the unlink lands later,
				// when the countdown runs out.
				if a.unlink[id] == 0 {
					parent["checklist_specification_id"], parent["checklist_template_id"] = nil, nil
				}
				return http.StatusNoContent, nil
			}
			return 0, nil
		},
	})

	return a
}

// link is refused while the action has live associations; otherwise it
// replaces any link and seeds the fail path when it is null, and answers 200
// with a body no client reads, sent here without it (observed against the
// API). A version that does not exist or is deleted answers 404.
func (a *Approvals) link(action Item, specID string) (int, *Refusal) {
	id := idString(action)
	live := 0
	for _, assoc := range a.associations[id] {
		if !assoc.archived {
			live++
		}
	}
	if live > 0 {
		// Without ReleasePoliciesAfter the count stays at zero: the guard only
		// keeps it from going negative, which would change nothing.
		if a.release[id] > 0 {
			if a.release[id]--; a.release[id] == 0 {
				a.dropLive(id, "")
			}
		}
		return 0, problem(http.StatusConflict, "Approval action already has policies; cannot also assign a checklist specification",
			fmt.Sprintf("Approval action %s has %d policy/policies; cannot assign a checklist specification", id, live))
	}
	if col, ok := a.s.collections[ChecklistSpecification]; ok {
		if spec, found := col.items[specID]; !found || Str(spec, "status") == "deleted" {
			return 0, problem(http.StatusNotFound, "Checklist specification not found", nil)
		}
	}
	if action["on_checklist_fail"] == nil {
		action["on_checklist_fail"] = "pending"
		if Str(action, "on_policy_fail") == "deny" {
			action["on_checklist_fail"] = "deny"
		}
	}
	action["checklist_specification_id"], action["checklist_template_id"] = specID, specID
	return http.StatusOK, nil
}

// ReleasePoliciesAfter makes an action's live associations go after that many
// links refused for them, as if their DELETEs landed between the attempts:
// one apply that removes the associations and links a specification, made
// deterministic. Test arrangement.
func (a *Approvals) ReleasePoliciesAfter(actionID string, attempts int) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.release[actionID] = attempts
}

// UnlinkAfter makes an action's link go after that many associations refused
// for it, as if the unlink landed between the attempts: one apply that
// removes the link and adds associations, made deterministic. An unlink
// request before then answers 204 and lands with the countdown. Test
// arrangement.
func (a *Approvals) UnlinkAfter(actionID string, attempts int) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.unlink[actionID] = attempts
}

// validateChecklistFail is the API's check of the fail path: null or one of
// deny, pending, manual. Its message is not observed against the API.
func validateChecklistFail(value any) *Refusal {
	switch value {
	case nil, "deny", "pending", "manual":
		return nil
	}
	return Refuse("The checklist fail path is not valid. It must be one of the following: deny, pending, manual")
}

// dropLive destroys an action's live association with policyID, or every live
// one when policyID is "".
func (a *Approvals) dropLive(actionID, policyID string) {
	var kept []*association
	for _, assoc := range a.associations[actionID] {
		if assoc.archived || (policyID != "" && assoc.policyID != policyID) {
			kept = append(kept, assoc)
		}
	}
	a.associations[actionID] = kept
}

// policiesOf is the action's `policies` as GET answers it: every associated
// active policy, live or archived. The archived ones are still listed after a
// migration, linked or unlinked, and a policy associated again over its
// archived row is listed once (observed against the API).
func (a *Approvals) policiesOf(actionID string) []Item {
	policies := []Item{}
	listed := map[string]bool{}
	for _, assoc := range a.associations[actionID] {
		policy, ok := a.s.collections[ApprovalPolicy].items[assoc.policyID]
		if !ok || (Str(policy, "status") != "" && Str(policy, "status") != "active") || listed[assoc.policyID] {
			continue
		}
		listed[assoc.policyID] = true
		policies = append(policies, Item{
			"id": policy["id"], "nrn": policy["nrn"], "name": policy["name"],
			"conditions": policy["conditions"], "selector": policy["selector"],
		})
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i]["id"].(int) < policies[j]["id"].(int) })
	return policies
}

func idString(item Item) string { return fmt.Sprint(item["id"]) }

// LivePolicies is the policies of an action's live associations — test
// inspection, as Items is: the GET cannot tell them from the archived ones.
func (a *Approvals) LivePolicies(actionID string) []string {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	live := []string{}
	for _, assoc := range a.associations[actionID] {
		if !assoc.archived {
			live = append(live, assoc.policyID)
		}
	}
	return live
}

// SeedAssociation arranges an association row, bypassing hooks — test
// arrangement, as Seed is.
func (a *Approvals) SeedAssociation(actionID, policyID string, archived bool) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.associations[actionID] = append(a.associations[actionID], &association{policyID: policyID, archived: archived})
}

// Migrate arranges what a migration with the API leaves (observed against the
// API): the action linked to specID, its on_checklist_fail seeded as a link
// seeds it, and every live association archived. The migration itself is not
// an endpoint of this fake.
func (a *Approvals) Migrate(actionID, specID string) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	action := a.s.collections[ApprovalAction].items[actionID]
	action["checklist_specification_id"] = specID
	action["checklist_template_id"] = specID
	if action["on_checklist_fail"] == nil {
		action["on_checklist_fail"] = "pending"
		if Str(action, "on_policy_fail") == "deny" {
			action["on_checklist_fail"] = "deny"
		}
	}
	for _, assoc := range a.associations[actionID] {
		assoc.archived = true
	}
}

package fakeplatform

// addOwnerToLinkableTo mirrors the API's create default for services: the
// owning entity_nrn is appended to linkable_to, deduplicated, keeping the
// caller's order (main-service-api ServiceInstanceService
// #addDefaultsFromSpecification: [...new Set([...linkableTo, entityNrn])]).
// Only create does this; a PATCH replaces linkable_to as sent, owner or not.
func addOwnerToLinkableTo(item Item) {
	owner := Str(item, "entity_nrn")
	linkable, _ := item["linkable_to"].([]any)
	for _, nrn := range linkable {
		if nrn == owner {
			return
		}
	}
	item["linkable_to"] = append(linkable, owner)
}

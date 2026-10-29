package fakeplatform

import (
	"fmt"
	"net/http"
)

// PackageInstallationContract is the API's installation rules for one test:
// which revisions belong to the installed package (and their versions), the
// package default an install without default_revision_id resolves to, and how
// many live services each installation has born (what refuses the uninstall).
// Fields may change between steps: hooks read them at request time.
type PackageInstallationContract struct {
	Revisions         map[string]string
	DefaultRevisionID string
	LiveServices      map[string]int
}

// RegisterPackageInstallation mounts /package_installations with the rules
// services-api enforces (CASES-MATRIX §9g): the revision must be the package's,
// an omitted revision resolves to the package default at install time, an
// omitted visible_to is [nrn], and a delete is refused while a service born
// from the installation is alive. Messages are the API's own.
func RegisterPackageInstallation(s *Server, contract *PackageInstallationContract) {
	revision := func(item Item, revisionID string) *Refusal {
		version, ok := contract.Revisions[revisionID]
		if !ok {
			return Refuse("Revision %s does not belong to package %v", revisionID, item["package_id"])
		}
		item["default_revision_id"] = revisionID
		item["default_version"] = version
		return nil
	}

	s.RegisterWith("package_installations", "inst", Options{CreateStatus: http.StatusCreated}, Hooks{
		OnCreate: func(_ *Server, item Item) *Refusal {
			revisionID, _ := item["default_revision_id"].(string)
			if revisionID == "" {
				revisionID = contract.DefaultRevisionID
			}
			if refusal := revision(item, revisionID); refusal != nil {
				return refusal
			}
			if _, set := item["visible_to"]; !set {
				item["visible_to"] = []any{item["nrn"]}
			}
			item["created_by"] = "3"
			item["missing_optional_requirements"] = []any{Item{"category": "scope-configurations"}}
			return nil
		},
		// The API answers missing_optional_requirements only on the install response.
		OnGet: func(_ *Server, item Item) {
			delete(item, "missing_optional_requirements")
		},
		OnPatch: func(_ *Server, existing, patch Item) *Refusal {
			if revisionID, ok := patch["default_revision_id"].(string); ok && revisionID != "" {
				if refusal := revision(existing, revisionID); refusal != nil {
					return refusal
				}
			}
			if visibleTo, ok := patch["visible_to"]; ok {
				existing["visible_to"] = visibleTo
			}
			return nil
		},
		OnDelete: func(_ *Server, item Item) *Refusal {
			id := fmt.Sprint(item["id"])
			if born := contract.LiveServices[id]; born > 0 {
				return Refuse("Package installation %s cannot be deleted: %d service instance(s) under its visible_to were born from it", id, born)
			}
			return nil
		},
	})
}

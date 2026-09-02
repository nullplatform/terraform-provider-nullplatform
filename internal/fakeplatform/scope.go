package fakeplatform

import "strings"

type ScopeLog struct {
	NrnPatches []Item
}

// RegisterScope mounts /scope and /nrn, with the NRN seeded at nrn holding awsNamespace
// under namespaces.aws — the shape GetNRN reads. PATCH bodies land in the returned log.
func RegisterScope(s *Server, nrn string, awsNamespace Item) *ScopeLog {
	log := &ScopeLog{}

	s.RegisterWith("scope", "scope", Options{NumericIDs: true}, Hooks{
		OnCreate: func(_ *Server, item Item) *Refusal {
			item["nrn"] = nrn
			item["status"] = "active"
			return nil
		},
	})

	s.Register("nrn", "nrn", Hooks{
		OnPatch: func(_ *Server, existing, patch Item) *Refusal {
			log.NrnPatches = append(log.NrnPatches, patch)

			namespaces, _ := existing["namespaces"].(Item)
			aws, _ := namespaces["aws"].(Item)

			for key, value := range patch {
				if field, ok := strings.CutPrefix(key, "aws."); ok {
					aws[field] = value
				}
			}

			return nil
		},
	})

	s.Seed("nrn", nrn, Item{
		"nrn":        nrn,
		"namespaces": Item{"aws": awsNamespace},
	})

	return log
}

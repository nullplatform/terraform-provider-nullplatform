package fakeplatform

import (
	"fmt"
	"strings"
)

// NrnLog records every PATCH body sent to /nrn, as the client sent it; a
// non-nil Refuse rejects the next patches.
type NrnLog struct {
	Patches []Item
	Refuse  *Refusal
}

// RegisterScope mounts /scope and /nrn. The API mints a scope's NRN on create;
// NRN keys are PATCHed flat ("aws.log_group_name") and read back nested under
// namespaces.<ns>, which is how GET /nrn/<nrn>?ids=... answers. A fresh NRN
// holds no namespaces.
func RegisterScope(s *Server) *NrnLog {
	log := &NrnLog{}

	s.RegisterWith("scope", "scope", Options{NumericIDs: true}, Hooks{
		OnCreate: func(s *Server, item Item) *Refusal {
			nrn := fmt.Sprintf("organization=1:account=2:namespace=3:application=%v:scope=%d",
				item["application_id"], s.collections["scope"].seq+1)
			item["nrn"] = nrn
			s.collections["nrn"].items[nrn] = Item{"nrn": nrn, "namespaces": Item{}}
			return nil
		},
	})

	s.Register("nrn", "nrn", Hooks{
		OnPatch: func(_ *Server, existing, patch Item) *Refusal {
			log.Patches = append(log.Patches, patch)
			if log.Refuse != nil {
				return log.Refuse
			}
			namespaces := existing["namespaces"].(Item)
			for key, value := range patch {
				ns, field, _ := strings.Cut(key, ".")
				bucket, ok := namespaces[ns].(Item)
				if !ok {
					bucket = Item{}
					namespaces[ns] = bucket
				}
				bucket[field] = value
			}
			return nil
		},
	})

	return log
}

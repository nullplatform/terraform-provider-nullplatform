package fakeplatform

import "net/http"

type ApiKeyLog struct {
	Internal []any
}

func RegisterApiKey(s *Server) *ApiKeyLog {
	log := &ApiKeyLog{}

	s.RegisterWith("api_key", "apikey", Options{
		NumericIDs:   true,
		CreateStatus: http.StatusCreated,
	}, Hooks{
		OnCreate: func(_ *Server, item Item) *Refusal {
			log.Internal = append(log.Internal, item["internal"])

			delete(item, "internal")

			item["api_key"] = "1.c2VjcmV0"
			item["masked_api_key"] = "1.cxxxxxcret"

			return nil
		},
		OnGet: func(_ *Server, item Item) {
			delete(item, "api_key")
		},
	})

	return log
}

package fakeplatform

import "net/http"

// RegisterToken mounts /token: POST {"apikey"} exchanges apiKey for
// accessToken and answers 401 for any other key. Every exchange is kept, so
// Items("token") counts the tokens issued.
func RegisterToken(s *Server, apiKey, accessToken string) {
	s.Register("token", "token", Hooks{
		OnCreate: func(_ *Server, item Item) *Refusal {
			if Str(item, "apikey") != apiKey {
				return &Refusal{Status: http.StatusUnauthorized, Message: "Unauthorized"}
			}
			delete(item, "apikey")
			item["access_token"] = accessToken
			item["refresh_token"] = "refresh-token"
			return nil
		},
	})
}

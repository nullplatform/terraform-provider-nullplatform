package nullplatform

type TokenRequest struct {
	Apikey string `json:"apikey"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

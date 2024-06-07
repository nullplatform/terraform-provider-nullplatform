package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
)

const TOKEN_PATH = "/token"

type TokenRequest struct {
	Apikey string `json:"apikey"`
}

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type NullClient struct {
	Client *http.Client
	ApiURL string
	ApiKey string
	Token  Token
}

type NullErrors struct {
	Message string `json:"message"`
	Id      int    `json:"id"`
}

type NullOps interface {
	GetToken() diag.Diagnostics

	CreateScope(*Scope) (*Scope, error)
	PatchScope(string, *Scope) error
	GetScope(string) (*Scope, error)

	PatchNRN(string, *PatchNRN) error
	GetNRN(string) (*NRN, error)

	GetApplication(appId string) (*Application, error)

	CreateService(*Service) (*Service, error)
	GetService(string) (*Service, error)
	PatchService(string, *Service) error
	DeleteService(string) error

	CreateLink(*Link) (*Link, error)
	PatchLink(string, *Link) error
	DeleteLink(string) error
	GetLink(string) (*Link, error)

	CreateParameter(param *Parameter, importIfCreated bool) (*Parameter, error)
	PatchParameter(parameterId string, param *Parameter) error
	GetParameter(parameterId string) (*Parameter, error)
	DeleteParameter(parameterId string) error
	GetParameterList(nrn string) (*ParameterList, error)

	CreateParameterValue(paramId int, paramValue *ParameterValue) (*ParameterValue, error)
	GetParameterValue(parameterId string, parameterValueId string) (*ParameterValue, error)
	DeleteParameterValue(parameterId string, parameterValueId string) error
}

func (c *NullClient) GetToken() diag.Diagnostics {
	treq := TokenRequest{
		Apikey: c.ApiKey,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(treq)

	if err != nil {
		return diag.FromErr(err)
	}

	log.Print("\n\n--- Fetching access token... ---\n\n")

	r, err := http.NewRequest("POST", fmt.Sprintf("https://%s%s", c.ApiURL, TOKEN_PATH), &buf)
	if err != nil {
		return diag.FromErr(err)
	}

	r.Header.Add("Content-Type", "application/json")

	res, err := c.Client.Do(r)
	if err != nil {
		return diag.FromErr(err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return diag.FromErr(fmt.Errorf("error creating resource, got %d, api key was %s", res.StatusCode, c.ApiKey))
	}

	tRes := &Token{}
	derr := json.NewDecoder(res.Body).Decode(tRes)

	if derr != nil {
		return diag.FromErr(derr)
	}

	if tRes.AccessToken == "" {
		return diag.FromErr(fmt.Errorf("no access token for null platform token rsp is: %s", tRes))
	}

	c.Token = (*tRes)

	return nil
}

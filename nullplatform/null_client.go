package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

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
	Client     *http.Client
	ApiURL     string
	ApiKey     string
	Token      Token
	tokenMutex sync.Mutex
}

type NullErrors struct {
	Message string `json:"message"`
	Id      int    `json:"id"`
}

type NullOps interface {
	MakeRequest(method, path string, body *bytes.Buffer) (*http.Response, error)

	CreateScope(*Scope) (*Scope, error)
	PatchScope(string, *Scope) error
	GetScope(string) (*Scope, error)
	DeleteScope(string) error

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
	GetParameter(parameterId string, nrn *string) (*Parameter, error)
	DeleteParameter(parameterId string) error
	GetParameterList(nrn string, hideValues ...bool) (*ParameterList, error)

	CreateParameterValue(paramId int, paramValue *ParameterValue) (*ParameterValue, error)
	GetParameterValue(parameterId string, parameterValueId string, nrn *string) (*ParameterValue, error)
	DeleteParameterValue(parameterId string, parameterValueId string) error

	CreateApprovalAction(action *ApprovalAction) (*ApprovalAction, error)
	PatchApprovalAction(approvalActionId string, action *ApprovalAction) error
	GetApprovalAction(approvalActionId string) (*ApprovalAction, error)
	DeleteApprovalAction(approvalActionId string) error

	CreateApprovalPolicy(policy *ApprovalPolicy) (*ApprovalPolicy, error)
	PatchApprovalPolicy(ApprovalPolicyId string, policy *ApprovalPolicy) error
	GetApprovalPolicy(ApprovalPolicyId string) (*ApprovalPolicy, error)
	DeleteApprovalPolicy(ApprovalPolicyId string) error

	AssociatePolicyWithAction(approvalActionId, approvalPolicyID string) error
	DisassociatePolicyFromAction(approvalActionId, approvalPolicyID string) error

	CreateNotificationChannel(notification *NotificationChannel) (*NotificationChannel, error)
	GetNotificationChannel(notificationId string) (*NotificationChannel, error)
	DeleteNotificationChannel(notificationId string) error

	CreateRuntimeConfiguration(rc *RuntimeConfiguration) (*RuntimeConfiguration, error)
	PatchRuntimeConfiguration(runtimeConfigId string, rc *RuntimeConfiguration) error
	GetRuntimeConfiguration(runtimeConfigId string) (*RuntimeConfiguration, error)
	DeleteRuntimeConfiguration(runtimeConfigId string) error

	CreateProviderConfig(p *ProviderConfig) (*ProviderConfig, error)
	GetProviderConfig(providerConfigId string) (*ProviderConfig, error)
	PatchProviderConfig(providerConfigId string, p *ProviderConfig) error
	DeleteProviderConfig(providerConfigId string) error
	GetSpecificationIdFromSlug(slug string) (string, error)
	GetSpecificationSlugFromId(id string) (string, error)
}

func (c *NullClient) MakeRequest(method, path string, body *bytes.Buffer) (*http.Response, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	var req *http.Request
	var err error
	url := fmt.Sprintf("https://%s%s", c.ApiURL, path)

	if body != nil {
		req, err = http.NewRequest(method, url, body)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	return c.Client.Do(req)
}

func (c *NullClient) ensureValidToken() error {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	if c.Token.AccessToken == "" {
		diag := c.getToken()
		if diag != nil {
			return fmt.Errorf(diag[0].Summary)
		}
	}

	return nil
}

func (c *NullClient) getToken() diag.Diagnostics {
	treq := TokenRequest{
		Apikey: c.ApiKey,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(treq)

	if err != nil {
		return diag.FromErr(err)
	}

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
		return diag.FromErr(fmt.Errorf("failed to get access token, got %d", res.StatusCode))
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

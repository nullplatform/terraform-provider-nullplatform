package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
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
	Client          *http.Client
	ApiURL          string
	ApiKey          string
	Token           Token
	tokenMutex      sync.Mutex
	cachedOrgID     string
	cachedOrgIDLock sync.RWMutex
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
	UpdateNotificationChannel(notificationId string, notification *NotificationChannel) error
	DeleteNotificationChannel(notificationId string) error

	CreateRuntimeConfiguration(rc *RuntimeConfiguration) (*RuntimeConfiguration, error)
	PatchRuntimeConfiguration(runtimeConfigId string, rc *RuntimeConfiguration) error
	GetRuntimeConfiguration(runtimeConfigId string) (*RuntimeConfiguration, error)
	DeleteRuntimeConfiguration(runtimeConfigId string) error

	CreateProviderConfig(p *ProviderConfig) (*ProviderConfig, error)
	GetProviderConfig(providerConfigId string) (*ProviderConfig, error)
	PatchProviderConfig(providerConfigId string, p *ProviderConfig) error
	DeleteProviderConfig(providerConfigId string) error
	GetSpecificationIdFromSlug(slug string, nrn string) (string, error)
	GetSpecificationSlugFromId(id string) (string, error)

	GetOrganizationIDFromToken() (string, error)
	GetAccountBySlug(organizationID, slug string) (map[string]interface{}, error)
	GetNamespaceBySlug(accountID, slug string) (map[string]interface{}, error)
	GetApplicationBySlug(namespaceID, slug string) (map[string]interface{}, error)
	GetScopeBySlug(applicationID, slug string) (map[string]interface{}, error)

	CreateDimension(*Dimension) (*Dimension, error)
	GetDimension(*string, *string, *string, *string, *string) (*Dimension, error)
	UpdateDimension(string, *Dimension) error
	DeleteDimension(string) error

	CreateDimensionValue(dv *DimensionValue) (*DimensionValue, error)
	GetDimensionValue(dimensionID, valueID int) (*DimensionValue, error)
	DeleteDimensionValue(dimensionID, valueID int) error

	CreateAccount(account *Account) (*Account, error)
	GetAccount(accountId string) (*Account, error)
	PatchAccount(accountId string, account *Account) error
	DeleteAccount(accountId string) error

	CreateNamespace(namespace *Namespace) (*Namespace, error)
	GetNamespace(namespaceId string) (*Namespace, error)
	PatchNamespace(namespaceId string, account *Namespace) error
	DeleteNamespace(namespaceId string) error

	CreateMetadataSpecification(spec *MetadataSpecification) (*MetadataSpecification, error)
	GetMetadataSpecification(specId string) (*MetadataSpecification, error)
	UpdateMetadataSpecification(specId string, spec *MetadataSpecification) (*MetadataSpecification, error)
	DeleteMetadataSpecification(specId string) error

	CreateServiceSpecification(s *ServiceSpecification) (*ServiceSpecification, error)
	GetServiceSpecification(specId string) (*ServiceSpecification, error)
	PatchServiceSpecification(specId string, s *ServiceSpecification) error
	DeleteServiceSpecification(specId string) error

	CreateLinkSpecification(s *LinkSpecification) (*LinkSpecification, error)
	GetLinkSpecification(specId string) (*LinkSpecification, error)
	PatchLinkSpecification(specId string, s *LinkSpecification) error
	DeleteLinkSpecification(specId string) error

	CreateActionSpecification(s *ActionSpecification) (*ActionSpecification, error)
	GetActionSpecification(specId string, parentType string, parentId string) (*ActionSpecification, error)
	PatchActionSpecification(specId string, s *ActionSpecification, parentType string, parentId string) error
	DeleteActionSpecification(specId string, parentType string, parentId string) error

	GetApiKey(apiKeyId int64) (*ApiKey, error)
	CreateApiKey(body *CreateApiKeyRequestBody) (*CreateApiKeyResponseBody, error)
	PatchApiKey(apiKeyId int64, body *PatchApiKeyRequestBody) error
	DeleteApiKey(apiKeyId int64) error

	CreateUser(u *User) (*User, error)
	GetUser(userID string) (*User, error)
	UpdateUser(userID string, u *User) error
	DeleteUser(userID string) error

	CreateAuthzGrant(grant *AuthzGrant) (*AuthzGrant, error)
	GetAuthzGrant(grantID string) (*AuthzGrant, error)
	DeleteAuthzGrant(grantID string) error

	CreateTechnologyTemplate(t *TechnologyTemplate) (*TechnologyTemplate, error)
	GetTechnologyTemplate(templateId string) (*TechnologyTemplate, error)
	PatchTechnologyTemplate(templateId string, t *TechnologyTemplate) error

	CreateMetadata(entity, entityId, metadataType string, m *Metadata) error
	GetMetadata(entity, entityId, metadataType string) (*Metadata, error)
	UpdateMetadata(entity, entityId, metadataType string, m *Metadata) error
	DeleteMetadata(entity, entityId, metadataType string) error
}

func (c *NullClient) PrepareQueryString(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}

	var query string
	// params is already validated outside, here it is assumed it is a non empty map of strings
	for k, v := range params {
		query = strings.Join([]string{query, strings.Join([]string{k, v}, "=")}, "&")
	}

	return "?" + query
}

type RetryConfig struct {
	MaxRetries      int           // Maximum number of retry attempts
	InitialInterval time.Duration // Initial interval between retries
	MaxInterval     time.Duration // Maximum interval between retries
	Multiplier      float64       // Multiplier for exponential backoff
	RandomFactor    float64       // Random factor for jitter
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		RandomFactor:    0.1,
	}
}

func isRetryableClientError(err error) bool {
	if err == nil {
		return false
	}

	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	switch err.(type) {
	case *net.OpError:
		return true
	case *net.DNSError:
		return true
	default:
		return false
	}
}

func (rc *RetryConfig) calculateBackoff(attempt int) time.Duration {
	backoff := float64(rc.InitialInterval) * math.Pow(rc.Multiplier, float64(attempt))

	if backoff > float64(rc.MaxInterval) {
		backoff = float64(rc.MaxInterval)
	}

	delta := rc.RandomFactor * backoff
	minBackoff := backoff - delta
	maxBackoff := backoff + delta

	return time.Duration(minBackoff + rand.Float64()*(maxBackoff-minBackoff))
}

func (c *NullClient) MakeRequest(method, path string, body *bytes.Buffer) (*http.Response, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	retryConfig := DefaultRetryConfig()
	var lastErr error

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := retryConfig.calculateBackoff(attempt - 1)
			time.Sleep(backoff)
		}

		var req *http.Request
		var err error
		url := fmt.Sprintf("https://%s%s", c.ApiURL, path)

		if body != nil {
			bodyCopy := bytes.NewBuffer(body.Bytes())
			req, err = http.NewRequest(method, url, bodyCopy)
		} else {
			req, err = http.NewRequest(method, url, nil)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

		resp, err := c.Client.Do(req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isRetryableClientError(err) {
			return nil, fmt.Errorf("non-retryable error: %v", err)
		}
	}

	return nil, fmt.Errorf("max retries exceeded, last error: %v", lastErr)
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

func (c *NullClient) GetOrganizationIDFromToken() (string, error) {
	c.cachedOrgIDLock.RLock()
	if c.cachedOrgID != "" {
		defer c.cachedOrgIDLock.RUnlock()
		return c.cachedOrgID, nil
	}
	c.cachedOrgIDLock.RUnlock()

	if err := c.ensureValidToken(); err != nil {
		return "", fmt.Errorf("failed to ensure valid token: %v", err)
	}

	c.cachedOrgIDLock.Lock()
	defer c.cachedOrgIDLock.Unlock()

	if c.cachedOrgID != "" {
		return c.cachedOrgID, nil
	}

	token, _, err := new(jwt.Parser).ParseUnverified(c.Token.AccessToken, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims type: %T", token.Claims)
	}

	groups, ok := claims["cognito:groups"]
	if !ok {
		return "", fmt.Errorf("claim was not found")
	}

	groupsSlice, ok := groups.([]interface{})
	if !ok {
		return "", fmt.Errorf("cognito:groups is not a slice: %T", groups)
	}

	for _, group := range groupsSlice {
		groupStr, ok := group.(string)
		if !ok {
			log.Printf("Unexpected group type: %T", group)
			continue
		}
		if strings.HasPrefix(groupStr, "@nullplatform/organization=") {
			orgID := strings.TrimPrefix(groupStr, "@nullplatform/organization=")
			c.cachedOrgID = orgID
			return orgID, nil
		}
	}

	return "", fmt.Errorf("organization ID not found in token")
}

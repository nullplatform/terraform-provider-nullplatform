package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const API_KEY_PATH = "/api_key"

type ApiKey struct {
	ID           int64             `json:"id"`
	Name         string            `json:"name"`
	MaskedApiKey string            `json:"masked_api_key"`
	Tags         []Tag             `json:"tags"`
	Grants       []ApiKeyGrantRead `json:"grants"`
	OwnerID      *int64            `json:"owner_id"`
	LastUsedAt   *string           `json:"last_used_at"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

// ApiKeyGrant is one grant as the API accepts it. Every item carries an NRN
// plus exactly one shape: an existing role by ID or slug, a literal list of
// actions, or a set of roles to inherit. The API's oneOf rejects a mix.
//
// Actions is deliberately untyped: on its own it is []string, and alongside
// Inherits it is instead a {add, remove} object applied to the union of the
// inherited roles. HCL has no union types, so the provider's schema splits
// that into actions / add_actions / remove_actions and reassembles it here.
type ApiKeyGrant struct {
	NRN      string   `json:"nrn"`
	RoleID   *int64   `json:"role_id,omitempty"`
	RoleSlug *string  `json:"role_slug,omitempty"`
	Actions  any      `json:"actions,omitempty"`
	Inherits []string `json:"inherits,omitempty"`
}

// ApiKeyGrantRead is the same grant as the API returns it, which is not the
// shape it was written in. A grant created from actions or inherits resolves
// to a role private to the key, and reads back as the action names it stands
// for: Actions holds the effective set, and the inherited form additionally
// reports the roles it merged plus the delta applied to them.
type ApiKeyGrantRead struct {
	NRN      string                `json:"nrn"`
	RoleID   *int64                `json:"role_id"`
	RoleSlug *string               `json:"role_slug"`
	Actions  []string              `json:"actions"`
	Inherits []ApiKeyInheritedRole `json:"inherits"`
	Added    []string              `json:"added"`
	Removed  []string              `json:"removed"`
}

type ApiKeyInheritedRole struct {
	ID             int64  `json:"id"`
	Slug           string `json:"slug"`
	OrganizationID int64  `json:"organization_id"`
}

type CreateApiKeyResponseBody struct {
	ApiKey
	ApiKeyValue string `json:"api_key"`
}

type CreateApiKeyRequestBody struct {
	Name     string        `json:"name"`
	Grants   []ApiKeyGrant `json:"grants"`
	Tags     []Tag         `json:"tags,omitempty"`
	Internal *bool         `json:"internal,omitempty"`
}

type PatchApiKeyRequestBody struct {
	Name   string        `json:"name,omitempty"`
	Grants []ApiKeyGrant `json:"grants,omitempty"`
	Tags   []Tag         `json:"tags,omitempty"`
}

func (c *NullClient) GetApiKey(apiKeyId int64) (*ApiKey, error) {
	path := fmt.Sprintf("%s/%d", API_KEY_PATH, apiKeyId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get API Key resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	apiKey := &ApiKey{}
	err = json.NewDecoder(res.Body).Decode(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return apiKey, nil
}

func (c *NullClient) CreateApiKey(body *CreateApiKeyRequestBody) (*CreateApiKeyResponseBody, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode api key: %v", err)
	}

	res, err := c.MakeRequest("POST", API_KEY_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create API Key resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	apiKey := &CreateApiKeyResponseBody{}
	err = json.NewDecoder(res.Body).Decode(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return apiKey, nil
}

func (c *NullClient) PatchApiKey(apiKeyId int64, req *PatchApiKeyRequestBody) error {
	path := fmt.Sprintf("%s/%d", API_KEY_PATH, apiKeyId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*req)
	if err != nil {
		return fmt.Errorf("failed to encode api key: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to patch API Key resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) DeleteApiKey(apiKeyId int64) error {
	path := fmt.Sprintf("%s/%d", API_KEY_PATH, apiKeyId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete API Key resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

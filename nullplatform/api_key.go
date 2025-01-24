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
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	MaskedApiKey string        `json:"masked_api_key"`
	Tags         []Tag         `json:"tags"`
	Grants       []ApiKeyGrant `json:"grants"`
	OwnerID      *int64        `json:"owner_id"`
	LastUsedAt   *string       `json:"last_used_at"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

type ApiKeyGrant struct {
	NRN      string  `json:"nrn"`
	RoleID   *int64  `json:"role_id,omitempty"`
	RoleSlug *string `json:"role_slug,omitempty"`
}

type CreateApiKeyResponseBody struct {
	ApiKey
	ApiKeyValue string `json:"api_key"`
}

type CreateApiKeyRequestBody struct {
	Name   string        `json:"name"`
	Grants []ApiKeyGrant `json:"grants"`
	Tags   []Tag         `json:"tags,omitempty"`
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

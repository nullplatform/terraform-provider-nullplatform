package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	SCOPE_TYPE_PATH = "/scope_type"
)

type ScopeType struct {
	Id           int    `json:"id,omitempty"`
	Nrn          string `json:"nrn,omitempty"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Description  string `json:"description"`
	ProviderType string `json:"provider_type"`
	ProviderId   string `json:"provider_id"`
}

func (c *NullClient) CreateScopeType(s *ScopeType) (*ScopeType, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)

	if err != nil {
		return nil, fmt.Errorf("failed to encode scope type: %v", err)
	}

	res, err := c.MakeRequest("POST", SCOPE_TYPE_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create scope type resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	sRes := &ScopeType{}
	derr := json.NewDecoder(res.Body).Decode(sRes)

	if derr != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", derr)
	}

	return sRes, nil
}

func (c *NullClient) PatchScopeType(scopeTypeId string, s *ScopeType) error {
	path := fmt.Sprintf("%s/%s", SCOPE_TYPE_PATH, scopeTypeId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)

	if err != nil {
		return fmt.Errorf("failed to encode scope type: %v", err)
	}

	res, err := c.MakeRequest("PUT", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to patch scope type resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) GetScopeType(scopeTypeId string) (*ScopeType, error) {
	path := fmt.Sprintf("%s/%s", SCOPE_TYPE_PATH, scopeTypeId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get scope type resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	s := &ScopeType{}
	derr := json.NewDecoder(res.Body).Decode(s)

	if derr != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", derr)
	}

	return s, nil
}

func (c *NullClient) DeleteScopeType(scopeTypeId string) error {
	path := fmt.Sprintf("%s/%s", SCOPE_TYPE_PATH, scopeTypeId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) && (res.StatusCode != http.StatusNotFound) {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete scope type resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const SCOPE_DOMAIN_PATH = "/scope_domain"

type ScopeDomain struct {
	Id       string                 `json:"id,omitempty"`
	Name     string                 `json:"name,omitempty"`
	ScopeId  string                 `json:"scope_id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Status   string                 `json:"status,omitempty"`
	Selector map[string]interface{} `json:"selector,omitempty"`
}

func (c *NullClient) CreateScopeDomain(sd *ScopeDomain) (*ScopeDomain, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*sd); err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", SCOPE_DOMAIN_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode null error response: %w", err)
		}
		return nil, fmt.Errorf("error creating scope domain resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	sdRes := &ScopeDomain{}
	if err := json.NewDecoder(res.Body).Decode(sdRes); err != nil {
		return nil, err
	}

	return sdRes, nil
}

func (c *NullClient) GetScopeDomain(sdId string) (*ScopeDomain, error) {
	path := fmt.Sprintf("%s/%s", SCOPE_DOMAIN_PATH, sdId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting scope domain resource, got %d for %s", res.StatusCode, sdId)
	}

	sd := &ScopeDomain{}
	if err := json.NewDecoder(res.Body).Decode(sd); err != nil {
		return nil, err
	}

	return sd, nil
}

func (c *NullClient) PatchScopeDomain(sdId string, sd *ScopeDomain) error {
	path := fmt.Sprintf("%s/%s", SCOPE_DOMAIN_PATH, sdId)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*sd); err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode null error response: %w", err)
		}
		return fmt.Errorf("error updating scope domain resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

func (c *NullClient) DeleteScopeDomain(sdId string) error {
	path := fmt.Sprintf("%s/%s", SCOPE_DOMAIN_PATH, sdId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("error making DELETE request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		return fmt.Errorf("error deleting scope domain, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

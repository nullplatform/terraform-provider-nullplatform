package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ProviderSpecification struct {
	Id                string                 `json:"id,omitempty"`
	Name              string                 `json:"name,omitempty"`
	Slug              string                 `json:"slug,omitempty"`
	Icon              string                 `json:"icon,omitempty"`
	Description       string                 `json:"description,omitempty"`
	VisibleTo         []string               `json:"visible_to,omitempty"`
	SpecSchema        map[string]interface{} `json:"schema,omitempty"`
	AllowDimensions   bool                   `json:"allow_dimensions,omitempty"`
	DefaultDimensions map[string]interface{} `json:"default_dimensions,omitempty"`
	Category          string                 `json:"category,omitempty"`
	Categories        []string               `json:"categories,omitempty"`
	Dependencies      []string               `json:"dependencies,omitempty"`
	OrganizationId    *int                   `json:"organization_id,omitempty"`
}

func (c *NullClient) CreateProviderSpecification(s *ProviderSpecification) (*ProviderSpecification, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*s); err != nil {
		return nil, fmt.Errorf("failed to encode provider specification: %v", err)
	}

	res, err := c.MakeRequest("POST", SPECIFICATION_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create provider specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	sRes := &ProviderSpecification{}
	if err := json.NewDecoder(res.Body).Decode(sRes); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return sRes, nil
}

func (c *NullClient) GetProviderSpecification(specId string) (*ProviderSpecification, error) {
	path := fmt.Sprintf("%s/%s", SPECIFICATION_PATH, specId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get provider specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	spec := &ProviderSpecification{}
	if err := json.NewDecoder(res.Body).Decode(spec); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return spec, nil
}

func (c *NullClient) PatchProviderSpecification(specId string, s *ProviderSpecification) error {
	path := fmt.Sprintf("%s/%s", SPECIFICATION_PATH, specId)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*s); err != nil {
		return fmt.Errorf("failed to encode provider specification: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to patch provider specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) DeleteProviderSpecification(specId string) error {
	path := fmt.Sprintf("%s/%s", SPECIFICATION_PATH, specId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete provider specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	PROVIDER_CONFIG_PATH = "/provider"
	SPECIFICATION_PATH   = "/provider_specification"
)

type ProviderConfig struct {
	Id              string                 `json:"id,omitempty"`
	Nrn             string                 `json:"nrn,omitempty"`
	Dimensions      map[string]string      `json:"dimensions,omitempty"`
	SpecificationId string                 `json:"specificationId,omitempty"`
	Attributes      map[string]interface{} `json:"attributes,omitempty"`
}

type NpSpecification struct {
	Id   string `json:"id"`
	Slug string `json:"slug"`
}

type SpecificationResponse struct {
	Results []NpSpecification `json:"results"`
}

func (c *NullClient) CreateProviderConfig(p *ProviderConfig) (*ProviderConfig, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*p)

	if err != nil {
		return nil, fmt.Errorf("failed to encode provider config: %v", err)
	}

	res, err := c.MakeRequest("POST", PROVIDER_CONFIG_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create provider config resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	pRes := &ProviderConfig{}
	derr := json.NewDecoder(res.Body).Decode(pRes)

	if derr != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", derr)
	}

	return pRes, nil
}

func (c *NullClient) PatchProviderConfig(providerConfigId string, p *ProviderConfig) error {
	path := fmt.Sprintf("%s/%s", PROVIDER_CONFIG_PATH, providerConfigId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*p)

	if err != nil {
		return fmt.Errorf("failed to encode provider config: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to patch provider config resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) GetProviderConfig(providerConfigId string) (*ProviderConfig, error) {
	path := fmt.Sprintf("%s/%s", PROVIDER_CONFIG_PATH, providerConfigId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get provider config resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	p := &ProviderConfig{}
	derr := json.NewDecoder(res.Body).Decode(p)

	if derr != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", derr)
	}

	return p, nil
}

func (c *NullClient) DeleteProviderConfig(providerConfigId string) error {
	path := fmt.Sprintf("%s/%s", PROVIDER_CONFIG_PATH, providerConfigId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete provider config resource: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) GetSpecificationIdFromSlug(slug string, nrn string) (string, error) {
	path := fmt.Sprintf("%s?slug=%s&nrn=%s", SPECIFICATION_PATH, slug, nrn)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("failed to get specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	var specResponse SpecificationResponse
	derr := json.NewDecoder(res.Body).Decode(&specResponse)
	if derr != nil {
		return "", fmt.Errorf("failed to decode API response: %v", derr)
	}

	if len(specResponse.Results) == 0 {
		return "", fmt.Errorf("no specification found for slug: %s", slug)
	}

	return specResponse.Results[0].Id, nil
}

func (c *NullClient) GetSpecificationSlugFromId(id string) (string, error) {
	path := fmt.Sprintf("%s/%s", SPECIFICATION_PATH, id)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("failed to get specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	var specResponse SpecificationResponse
	derr := json.NewDecoder(res.Body).Decode(&specResponse)
	if derr != nil {
		return "", fmt.Errorf("failed to decode API response: %v", derr)
	}

	if len(specResponse.Results) == 0 {
		return "", fmt.Errorf("no specification found for id: %s", id)
	}

	return specResponse.Results[0].Slug, nil
}

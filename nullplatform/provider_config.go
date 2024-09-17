package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
		return nil, err
	}

	res, err := c.MakeRequest("POST", PROVIDER_CONFIG_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusBadRequest {
			nErr := &NullErrors{}
			dErr := json.NewDecoder(res.Body).Decode(nErr)
			if dErr != nil {
				return nil, fmt.Errorf("el error es %s", strings.ToLower(dErr.Error()))
			}
		}
		return nil, fmt.Errorf("error creating provider config resource, got status code: %d", res.StatusCode)
	}

	pRes := &ProviderConfig{}
	derr := json.NewDecoder(res.Body).Decode(pRes)

	if derr != nil {
		return nil, derr
	}

	return pRes, nil
}

func (c *NullClient) PatchProviderConfig(providerConfigId string, p *ProviderConfig) error {
	path := fmt.Sprintf("%s/%s", PROVIDER_CONFIG_PATH, providerConfigId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*p)

	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error patching provider config resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) GetProviderConfig(providerConfigId string) (*ProviderConfig, error) {
	path := fmt.Sprintf("%s/%s", PROVIDER_CONFIG_PATH, providerConfigId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	p := &ProviderConfig{}
	derr := json.NewDecoder(res.Body).Decode(p)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting provider config resource, got %d for %s", res.StatusCode, providerConfigId)
	}

	return p, nil
}

func (c *NullClient) DeleteProviderConfig(providerConfigId string) error {
	path := fmt.Sprintf("%s/%s", PROVIDER_CONFIG_PATH, providerConfigId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting provider config resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) GetSpecificationIdFromSlug(slug string) (string, error) {
	path := fmt.Sprintf("%s?slug=%s", SPECIFICATION_PATH, slug)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error getting specification, got status code: %d", res.StatusCode)
	}

	var specResponse SpecificationResponse
	derr := json.NewDecoder(res.Body).Decode(&specResponse)
	if derr != nil {
		return "", derr
	}

	if len(specResponse.Results) == 0 {
		return "", fmt.Errorf("no specification found for slug: %s", slug)
	}

	return specResponse.Results[0].Id, nil
}

func (c *NullClient) GetSpecificationSlugFromId(id string, nrn string) (string, error) {
	path := fmt.Sprintf("%s/%s&%s", SPECIFICATION_PATH, id, nrn)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error getting specification, got status code: %d", res.StatusCode)
	}

	var specResponse SpecificationResponse
	derr := json.NewDecoder(res.Body).Decode(&specResponse)
	if derr != nil {
		return "", derr
	}

	if len(specResponse.Results) == 0 {
		return "", fmt.Errorf("no specification found for id: %s and nrn: %s", id, nrn)
	}

	return specResponse.Results[0].Slug, nil
}

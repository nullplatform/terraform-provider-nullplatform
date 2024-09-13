package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const PROVIDER_PATH = "/provider"

type NpProvider struct {
	Id              string                 `json:"id,omitempty"`
	Nrn             string                 `json:"nrn,omitempty"`
	Dimensions      map[string]string      `json:"dimensions,omitempty"`
	SpecificationId string                 `json:"specificationId,omitempty"`
	Attributes      map[string]interface{} `json:"attributes,omitempty"`
}

func (c *NullClient) CreateNpProvider(p *NpProvider) (*NpProvider, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*p)

	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", PROVIDER_PATH, &buf)
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
		return nil, fmt.Errorf("error creating provider resource, got status code: %d", res.StatusCode)
	}

	pRes := &NpProvider{}
	derr := json.NewDecoder(res.Body).Decode(pRes)

	if derr != nil {
		return nil, derr
	}

	return pRes, nil
}

func (c *NullClient) PatchNpProvider(npProviderId string, p *NpProvider) error {
	path := fmt.Sprintf("%s/%s", PROVIDER_PATH, npProviderId)

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
		return fmt.Errorf("error patching provider resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) GetNpProvider(npProviderId string) (*NpProvider, error) {
	path := fmt.Sprintf("%s/%s", PROVIDER_PATH, npProviderId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	p := &NpProvider{}
	derr := json.NewDecoder(res.Body).Decode(p)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting provider resource, got %d for %s", res.StatusCode, npProviderId)
	}

	return p, nil
}

func (c *NullClient) DeleteNpProvider(npProviderId string) error {
	path := fmt.Sprintf("%s/%s", PROVIDER_PATH, npProviderId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting provider resource, got %d", res.StatusCode)
	}

	return nil
}

package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const RUNTIME_CONFIG_PATH = "/runtime_configuration"

type RuntimeConfiguration struct {
	Id         int                        `json:"id,omitempty"`
	Nrn        string                     `json:"nrn,omitempty"`
	Dimensions map[string]string          `json:"dimensions,omitempty"`
	Values     RuntimeConfigurationValues `json:"values,omitempty"`
}

type RuntimeConfigurationValues struct {
	AWS map[string]string `json:"aws,omitempty"`
}

func (c *NullClient) CreateRuntimeConfiguration(rc *RuntimeConfiguration) (*RuntimeConfiguration, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*rc)

	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", RUNTIME_CONFIG_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusBadRequest {
			nErr := &NullErrors{}
			dErr := json.NewDecoder(res.Body).Decode(nErr)
			if dErr != nil {
				return nil, fmt.Errorf("the error is %s", dErr)
			}
		}
		return nil, fmt.Errorf("error creating runtime configuration resource, got status code: %d", res.StatusCode)
	}

	rcRes := &RuntimeConfiguration{}
	derr := json.NewDecoder(res.Body).Decode(rcRes)

	if derr != nil {
		return nil, derr
	}

	return rcRes, nil
}

func (c *NullClient) PatchRuntimeConfiguration(runtimeConfigId string, rc *RuntimeConfiguration) error {
	path := fmt.Sprintf("%s/%s", RUNTIME_CONFIG_PATH, runtimeConfigId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*rc)

	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error patching runtime configuration resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) GetRuntimeConfiguration(runtimeConfigId string) (*RuntimeConfiguration, error) {
	path := fmt.Sprintf("%s/%s", RUNTIME_CONFIG_PATH, runtimeConfigId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	rc := &RuntimeConfiguration{}
	derr := json.NewDecoder(res.Body).Decode(rc)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting runtime configuration resource, got %d for %s", res.StatusCode, runtimeConfigId)
	}

	return rc, nil
}

func (c *NullClient) DeleteRuntimeConfiguration(runtimeConfigId string) error {
	path := fmt.Sprintf("%s/%s", RUNTIME_CONFIG_PATH, runtimeConfigId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting runtime configuration resource, got %d", res.StatusCode)
	}

	return nil
}

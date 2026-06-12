package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const CAPABILITY_PATH = "/capability"

type CapabilityEntity struct {
	Id          int                    `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Slug        string                 `json:"slug,omitempty"`
	Description string                 `json:"description,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Target      string                 `json:"target,omitempty"`
	Definition  map[string]interface{} `json:"definition,omitempty"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
}

func (c *NullClient) CreateCapability(capability *CapabilityEntity) (*CapabilityEntity, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*capability); err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", CAPABILITY_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode null error response: %w", err)
		}
		return nil, fmt.Errorf("error creating capability resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	capabilityRes := &CapabilityEntity{}
	if err := json.NewDecoder(res.Body).Decode(capabilityRes); err != nil {
		return nil, err
	}

	return capabilityRes, nil
}

func (c *NullClient) GetCapability(capabilityId string) (*CapabilityEntity, error) {
	path := fmt.Sprintf("%s/%s", CAPABILITY_PATH, capabilityId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting capability resource, got %d for %s", res.StatusCode, capabilityId)
	}

	capability := &CapabilityEntity{}
	if err := json.NewDecoder(res.Body).Decode(capability); err != nil {
		return nil, err
	}

	return capability, nil
}

func (c *NullClient) PatchCapability(capabilityId string, capability *CapabilityEntity) error {
	path := fmt.Sprintf("%s/%s", CAPABILITY_PATH, capabilityId)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*capability); err != nil {
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
		return fmt.Errorf("error updating capability resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

func (c *NullClient) DeleteCapability(capabilityId string) error {
	path := fmt.Sprintf("%s/%s", CAPABILITY_PATH, capabilityId)

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
		return fmt.Errorf("error deleting capability, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

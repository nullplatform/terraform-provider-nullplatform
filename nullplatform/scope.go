package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const SCOPE_PATH = "/scope"

type Capability struct {
	Visibility                 map[string]string `json:"visibility"`
	ServerlessRuntime          map[string]string `json:"serverless_runtime"`
	ServerlessHandler          map[string]string `json:"serverless_handler"`
	ServerlessTimeout          map[string]int    `json:"serverless_timeout"`
	ServerlessEphemeralStorage map[string]int    `json:"serverless_ephemeral_storage"`
	ServerlessMemory           map[string]int    `json:"serverless_memory"`
}

type RequestSpec struct {
	MemoryInGb   float32 `json:"memory_in_gb"`
	CpuProfile   string  `json:"cpu_profile"`
	LocalStorage int     `json:"local_storage"`
}

type Scope struct {
	Id               int         `json:"id"`
	Status           string      `json:"status"`
	Slug             string      `json:"slug"`
	Domain           string      `json:"domain"`
	ActiveDeployment int         `json:"active_deployment"`
	Nrn              string      `json:"nrn"`
	Name             string      `json:"name"`
	ApplicationId    int         `json:"application_id"`
	Type             string      `json:"type"`
	ExternalCreated  bool        `json:"external_created"`
	RequestedSpec    RequestSpec `json:"requested_spec"`
	Capabilities     Capability  `json:"capabilities"`
}

func (c *NullClient) CreateScope(s *Scope) (*Scope, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode((*s))

	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest("POST", fmt.Sprintf("https://%s%s", c.ApiURL, SCOPE_PATH), &buf)
	if err != nil {
		return nil, err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	sRes := &Scope{}
	derr := json.NewDecoder(res.Body).Decode(sRes)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error creating resource, got %d", res.StatusCode)
	}

	return sRes, nil
}

func (c *NullClient) PatchScope(scopeId string, s *Scope) error {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, SCOPE_PATH, scopeId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode((*s))

	if err != nil {
		return err
	}

	r, err := http.NewRequest("PATCH", url, &buf)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("error creating resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) GetScope(scopeId string) (*Scope, error) {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, SCOPE_PATH, scopeId)

	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	s := &Scope{}
	derr := json.NewDecoder(res.Body).Decode(s)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting resource, got %d for %s", res.StatusCode, scopeId)
	}

	return s, nil
}

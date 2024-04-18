package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

const SCOPE_PATH = "/scope"
const DUPLICATE_SCOPE_NAME_ERROR_STR = "There's already a scope with this name on this application"

type NullErrors struct {
	Message string `json:"message"`
	Id      int    `json:"id"`
}

type Capability struct {
	Visibility                 map[string]string `json:"visibility,omitempty"`
	ServerlessRuntime          map[string]string `json:"serverless_runtime,omitempty"`
	ServerlessHandler          map[string]string `json:"serverless_handler,omitempty"`
	ServerlessTimeout          map[string]int    `json:"serverless_timeout,omitempty"`
	ServerlessEphemeralStorage map[string]int    `json:"serverless_ephemeral_storage,omitempty"`
	ServerlessMemory           map[string]int    `json:"serverless_memory,omitempty"`
}

type RequestSpec struct {
	MemoryInGb   float32 `json:"memory_in_gb,omitempty"`
	CpuProfile   string  `json:"cpu_profile,omitempty"`
	LocalStorage int     `json:"local_storage,omitempty"`
}

type Scope struct {
	Id                    int               `json:"id,omitempty"`
	Status                string            `json:"status,omitempty"`
	Slug                  string            `json:"slug,omitempty"`
	Domain                string            `json:"domain,omitempty"`
	ActiveDeployment      int               `json:"active_deployment,omitempty"`
	Nrn                   string            `json:"nrn,omitempty"`
	Name                  string            `json:"name,omitempty"`
	ApplicationId         int               `json:"application_id,omitempty"`
	Type                  string            `json:"type,omitempty"`
	ExternalCreated       bool              `json:"external_created,omitempty"`
	RequestedSpec         *RequestSpec      `json:"requested_spec,omitempty"`
	Capabilities          *Capability       `json:"capabilities,omitempty"`
	Dimensions            map[string]string `json:"dimensions,omitempty"`
	RuntimeConfigurations []int             `json:"runtime_configurations,omitempty"`
}

func (c *NullClient) CreateScope(s *Scope) (*Scope, error) {
	url := fmt.Sprintf("https://%s%s", c.ApiURL, SCOPE_PATH)

	log.Printf("\n\n--- *** La URL es %s ---\n\n", url)
	log.Printf("\n\n--- *** El payload es %v ---\n\n", *s)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)

	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, err
	}

	io.Copy(os.Stdout, bytes.NewReader(buf.Bytes()))

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusBadRequest {
			nErr := &NullErrors{}
			dErr := json.NewDecoder(res.Body).Decode(nErr)
			if dErr != nil {
				fmt.Printf("El error es %s", dErr)
			}
			if (dErr == nil) && (nErr.Message == DUPLICATE_SCOPE_NAME_ERROR_STR) {
				return nil, fmt.Errorf(
					"error creating scope resource, duplicated scope name: %s found in null application id: %d",
					s.Name,
					s.ApplicationId,
				)
			}

		}
		io.Copy(os.Stdout, res.Body)
		return nil, fmt.Errorf("error creating scope resource, got status code: %d", res.StatusCode)
	}

	sRes := &Scope{}
	derr := json.NewDecoder(res.Body).Decode(sRes)

	if derr != nil {
		return nil, derr
	}

	return sRes, nil
}

func (c *NullClient) PatchScope(scopeId string, s *Scope) error {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, SCOPE_PATH, scopeId)

	log.Printf("\n\n--- *** La URL es %s ---\n\n", url)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)

	if err != nil {
		return err
	}

	r, err := http.NewRequest("PATCH", url, &buf)
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, bytes.NewReader(buf.Bytes()))

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		io.Copy(os.Stdout, res.Body)
		return fmt.Errorf("error patching scope resource, got %d", res.StatusCode)
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
		io.Copy(os.Stdout, res.Body)
		return nil, fmt.Errorf("error getting scope resource, got %d for %s", res.StatusCode, scopeId)
	}

	return s, nil
}

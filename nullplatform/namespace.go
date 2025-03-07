package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const NAMESPACE_PATH = "/namespace"

type Namespace struct {
	Id        int    `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	Slug      string `json:"slug,omitempty"`
	AccountId int    `json:"account_id,omitempty"`
}

func (c *NullClient) CreateNamespace(namespace *Namespace) (*Namespace, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*namespace)

	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", NAMESPACE_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode null error response: %w", err)
		}
		return nil, fmt.Errorf("error creating namespace resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	namespaceRes := &Namespace{}
	derr := json.NewDecoder(res.Body).Decode(namespaceRes)

	if derr != nil {
		return nil, derr
	}

	return namespaceRes, nil
}

func (c *NullClient) PatchNamespace(namespaceId string, namespace *Namespace) error {
	path := fmt.Sprintf("%s/%s", NAMESPACE_PATH, namespaceId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*namespace)

	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode null error response: %w", err)
		}
		return fmt.Errorf("error updating namespace resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

func (c *NullClient) GetNamespace(namespaceId string) (*Namespace, error) {
	path := fmt.Sprintf("%s/%s", NAMESPACE_PATH, namespaceId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	namespace := &Namespace{}
	derr := json.NewDecoder(res.Body).Decode(namespace)

	if derr != nil {
		return nil, derr
	}

	if namespace.Status == "deleted" {
		return namespace, fmt.Errorf("error getting namespace resource, the status is %s", namespace.Status)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting namespace resource, got %d for %s", res.StatusCode, namespaceId)
	}

	return namespace, nil
}

func (c *NullClient) DeleteNamespace(namespaceId string) error {
	path := fmt.Sprintf("%s/%s", NAMESPACE_PATH, namespaceId)

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
		return fmt.Errorf("error deleting namespace, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

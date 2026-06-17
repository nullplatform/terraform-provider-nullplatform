package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const DEPLOYMENT_STRATEGY_PATH = "/deployment_strategy"

type DeploymentStrategy struct {
	Id           int                    `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Nrn          string                 `json:"nrn,omitempty"`
	Dimensions   map[string]interface{} `json:"dimensions,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	ScopeTypeIds []string               `json:"scope_type_ids,omitempty"`
	CreatedBy    string                 `json:"created_by,omitempty"`
	UpdatedBy    string                 `json:"updated_by,omitempty"`
	CreatedAt    string                 `json:"created_at,omitempty"`
	UpdatedAt    string                 `json:"updated_at,omitempty"`
}

func (c *NullClient) CreateDeploymentStrategy(ds *DeploymentStrategy) (*DeploymentStrategy, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*ds); err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", DEPLOYMENT_STRATEGY_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode null error response: %w", err)
		}
		return nil, fmt.Errorf("error creating deployment strategy resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	dsRes := &DeploymentStrategy{}
	if err := json.NewDecoder(res.Body).Decode(dsRes); err != nil {
		return nil, err
	}

	return dsRes, nil
}

func (c *NullClient) GetDeploymentStrategy(dsId string) (*DeploymentStrategy, error) {
	path := fmt.Sprintf("%s/%s", DEPLOYMENT_STRATEGY_PATH, dsId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting deployment strategy resource, got %d for %s", res.StatusCode, dsId)
	}

	ds := &DeploymentStrategy{}
	if err := json.NewDecoder(res.Body).Decode(ds); err != nil {
		return nil, err
	}

	return ds, nil
}

func (c *NullClient) PatchDeploymentStrategy(dsId string, ds *DeploymentStrategy) error {
	path := fmt.Sprintf("%s/%s", DEPLOYMENT_STRATEGY_PATH, dsId)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*ds); err != nil {
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
		return fmt.Errorf("error updating deployment strategy resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

func (c *NullClient) DeleteDeploymentStrategy(dsId string) error {
	path := fmt.Sprintf("%s/%s", DEPLOYMENT_STRATEGY_PATH, dsId)

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
		return fmt.Errorf("error deleting deployment strategy, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

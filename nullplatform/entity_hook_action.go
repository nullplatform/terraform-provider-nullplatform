package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const ENTITY_HOOK_ACTION_PATH = "/entity_hook/action"

type EntityHookAction struct {
	Id              string            `json:"id,omitempty"`
	Nrn             string            `json:"nrn,omitempty"`
	Entity          string            `json:"entity,omitempty"`
	Action          string            `json:"action,omitempty"`
	Dimensions      map[string]string `json:"dimensions,omitempty"`
	OnPolicySuccess string            `json:"on_policy_success,omitempty"`
	OnPolicyFail    string            `json:"on_policy_fail,omitempty"`
	When            string            `json:"when,omitempty"`
	Type            string            `json:"type,omitempty"`
	On              string            `json:"on,omitempty"`
	Status          string            `json:"status,omitempty"`
}

func (c *NullClient) CreateEntityHookAction(action *EntityHookAction) (*EntityHookAction, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*action)

	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", ENTITY_HOOK_ACTION_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("error creating entity hook action resource, got status code: %d, %s", res.StatusCode, nErr.Message)
	}

	actionRes := &EntityHookAction{}
	derr := json.NewDecoder(res.Body).Decode(actionRes)

	if derr != nil {
		return nil, derr
	}

	return actionRes, nil
}

func (c *NullClient) PatchEntityHookAction(entityHookActionId string, action *EntityHookAction) error {
	path := fmt.Sprintf("%s/%s", ENTITY_HOOK_ACTION_PATH, entityHookActionId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*action)

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
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		return fmt.Errorf("error patching entity hook action resource, got status code: %d, %s", res.StatusCode, nErr.Message)
	}

	return nil
}

func (c *NullClient) GetEntityHookAction(entityHookActionId string) (*EntityHookAction, error) {
	path := fmt.Sprintf("%s/%s", ENTITY_HOOK_ACTION_PATH, entityHookActionId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	action := &EntityHookAction{}
	derr := json.NewDecoder(res.Body).Decode(action)

	if derr != nil {
		return nil, derr
	}

	if action.Status == "deleted" {
		return action, fmt.Errorf("error getting entity hook action resource, the status is %s", action.Status)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting entity hook action resource, got %d for %s", res.StatusCode, entityHookActionId)
	}

	return action, nil
}

func (c *NullClient) DeleteEntityHookAction(entityHookActionId string) error {
	path := fmt.Sprintf("%s/%s", ENTITY_HOOK_ACTION_PATH, entityHookActionId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting entity hook action resource, got %d", res.StatusCode)
	}

	return nil
}

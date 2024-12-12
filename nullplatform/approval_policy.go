package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const APPROVAL_POLICY_PATH = "/approval/policy"

type ApprovalPolicy struct {
	Id         int    `json:"id,omitempty"`
	Nrn        string `json:"nrn,omitempty"`
	Name       string `json:"name,omitempty"`
	Conditions string `json:"conditions,omitempty"`
	Status     string `json:"status,omitempty"`
}

func (c *NullClient) CreateApprovalPolicy(policy *ApprovalPolicy) (*ApprovalPolicy, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*policy)

	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", APPROVAL_POLICY_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode null error response: %w", err)
		}
		return nil, fmt.Errorf("error creating approval policy resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	policyRes := &ApprovalPolicy{}
	derr := json.NewDecoder(res.Body).Decode(policyRes)

	if derr != nil {
		return nil, derr
	}

	return policyRes, nil
}

func (c *NullClient) PatchApprovalPolicy(ApprovalPolicyId string, policy *ApprovalPolicy) error {
	path := fmt.Sprintf("%s/%s", APPROVAL_POLICY_PATH, ApprovalPolicyId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*policy)

	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error patching approval policy resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) GetApprovalPolicy(ApprovalPolicyId string) (*ApprovalPolicy, error) {
	path := fmt.Sprintf("%s/%s", APPROVAL_POLICY_PATH, ApprovalPolicyId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	policy := &ApprovalPolicy{}
	derr := json.NewDecoder(res.Body).Decode(policy)

	if derr != nil {
		return nil, derr
	}

	if policy.Status == "deleted" {
		return policy, fmt.Errorf("error getting approval policy resource, the status is %s", policy.Status)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting approval policy resource, got %d for %s", res.StatusCode, ApprovalPolicyId)
	}

	return policy, nil
}

func (c *NullClient) DeleteApprovalPolicy(approvalPolicyId string) error {
	path := fmt.Sprintf("%s/%s", APPROVAL_POLICY_PATH, approvalPolicyId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode null error response: %w", err)
		}
		return fmt.Errorf("error deleting approval policy resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

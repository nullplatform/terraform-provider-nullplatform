package nullplatform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const APPROVAL_ACTION_PATH = "/approval/action"

// errApprovalActionNotFound is a GET the API's router answered 404: an id that
// is not all digits, which no action has.
var errApprovalActionNotFound = errors.New("approval action not found")

// apiRouteNotFound is what the API's router answers for a path no route takes,
// not observed against the API. A missing numeric id answers 401 instead.
const apiRouteNotFound = "This is not a nullplatform.io entity or the HTTP verb is not supported."

type ApprovalAction struct {
	Id              int               `json:"id,omitempty"`
	Nrn             string            `json:"nrn,omitempty"`
	Entity          string            `json:"entity,omitempty"`
	Action          string            `json:"action,omitempty"`
	Dimensions      map[string]string `json:"dimensions,omitempty"`
	OnPolicySuccess string            `json:"on_policy_success,omitempty"`
	OnPolicyFail    string            `json:"on_policy_fail,omitempty"`
	OnChecklistFail string            `json:"on_checklist_fail,omitempty"`
	Status          string            `json:"status,omitempty"`
	Policies        []*ApprovalPolicy `json:"policies,omitempty"`
	// ChecklistSpecificationId is read-only: the link sets it.
	ChecklistSpecificationId string `json:"checklist_specification_id,omitempty"`
}

func (c *NullClient) CreateApprovalAction(action *ApprovalAction) (*ApprovalAction, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*action)

	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", APPROVAL_ACTION_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		defer res.Body.Close()
		return nil, fmt.Errorf("error creating approval action resource, got status code: %d, %s", res.StatusCode, redactMessage(nErr.Message))
	}

	actionRes := &ApprovalAction{}
	derr := json.NewDecoder(res.Body).Decode(actionRes)

	if derr != nil {
		return nil, derr
	}

	return actionRes, nil
}

func (c *NullClient) PatchApprovalAction(approvalActionId string, action *ApprovalAction) error {
	path := fmt.Sprintf("%s/%s", APPROVAL_ACTION_PATH, approvalActionId)

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
		return fmt.Errorf("error patching approval action resource, got %d: %s", res.StatusCode, apiErrorMessage(res))
	}

	return nil
}

// apiErrorMessage is the message of an error response, its credentials
// redacted: a gateway may echo the request. The body is decoded only for it
// (a problem document's "status" is a number); a body without one (a
// gateway's page) is the message as it came.
func apiErrorMessage(res *http.Response) string {
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var nErr NullErrors
	if json.Unmarshal(raw, &nErr) != nil || nErr.Message == "" {
		nErr.Message = strings.TrimSpace(string(raw))
	}
	return redactMessage(nErr.Message)
}

func (c *NullClient) GetApprovalAction(approvalActionId string) (*ApprovalAction, error) {
	path := fmt.Sprintf("%s/%s", APPROVAL_ACTION_PATH, approvalActionId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		message := apiErrorMessage(res)
		switch {
		// The router's text only travels with a 404; both are checked as the
		// contract says.
		case res.StatusCode == http.StatusNotFound && message == apiRouteNotFound:
			// Only an id that is not all digits gets here: the route takes
			// `:id([0-9]{1,})`. Any other 404 (a gateway's page) is not the API
			// saying the action is gone.
			return nil, fmt.Errorf("%w: %s", errApprovalActionNotFound, approvalActionId)
		case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
			// The API answers 401 for an id that does not exist, as it does for an
			// action this key cannot read: the two cannot be told apart.
			return nil, fmt.Errorf("approval action %s not found or not readable with this API key: %s", approvalActionId, message)
		}
		return nil, fmt.Errorf("error getting approval action resource, got %d for %s: %s", res.StatusCode, approvalActionId, message)
	}

	action := &ApprovalAction{}
	derr := json.NewDecoder(res.Body).Decode(action)

	if derr != nil {
		return nil, derr
	}

	if action.Status == "deleted" {
		return action, fmt.Errorf("error getting approval action resource, the status is %s", action.Status)
	}

	return action, nil
}

func (c *NullClient) DeleteApprovalAction(approvalActionId string) error {
	path := fmt.Sprintf("%s/%s", APPROVAL_ACTION_PATH, approvalActionId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting approval action resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) AssociatePolicyWithAction(approvalActionId, approvalPolicyID string) error {
	var buf bytes.Buffer
	path := fmt.Sprintf("%s/%s/policy", APPROVAL_ACTION_PATH, approvalActionId)

	policy := map[string]string{
		"policy_id": approvalPolicyID,
	}
	err := json.NewEncoder(&buf).Encode(policy)

	if err != nil {
		return err
	}

	res, err := c.MakeRequest("POST", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusCreated) {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		defer res.Body.Close()
		return &PolicyAssociationError{StatusCode: res.StatusCode, Message: redactMessage(nErr.Message)}
	}

	return nil
}

// PolicyAssociationError is a policy association the API refused; its text
// is the one the provider has always shown.
type PolicyAssociationError struct {
	StatusCode int
	Message    string
}

func (e *PolicyAssociationError) Error() string {
	return fmt.Sprintf("error associating approval policy with action, got status code: %d, %s", e.StatusCode, e.Message)
}

func (c *NullClient) DisassociatePolicyFromAction(approvalActionId, approvalPolicyID string) error {
	path := fmt.Sprintf("%s/%s/policy/%s", APPROVAL_ACTION_PATH, approvalActionId, approvalPolicyID)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting approval action and policy association, got %d", res.StatusCode)
	}

	return nil
}

// LinkChecklistSpecification links an approval action to a specification
// version; on a linked action the POST replaces the link.
func (c *NullClient) LinkChecklistSpecification(approvalActionId, specificationId string) error {
	return c.checklistSpecificationRequest("linking approval action "+approvalActionId+" to checklist specification "+specificationId,
		http.MethodPost, APPROVAL_ACTION_PATH+"/"+url.PathEscape(approvalActionId)+"/checklist_specification",
		map[string]string{"checklist_specification_id": specificationId}, http.StatusOK, nil)
}

// UnlinkChecklistSpecification unlinks an approval action; idempotent: the API
// answers 204 also when nothing is linked.
func (c *NullClient) UnlinkChecklistSpecification(approvalActionId string) error {
	return c.checklistSpecificationRequest("unlinking approval action "+approvalActionId+" from its checklist specification",
		http.MethodDelete, APPROVAL_ACTION_PATH+"/"+url.PathEscape(approvalActionId)+"/checklist_specification", nil, http.StatusNoContent, nil)
}

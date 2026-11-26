package nullplatform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const WORKFLOW_DEFINITIONS_PATH = "/workflows/definitions"

var (
	// errWorkflowNotFound is the API's 404 for a workflow it does not have.
	errWorkflowNotFound = errors.New("workflow not found")
	// errWorkflowAliasNotFound is the API's 404 for an alias the workflow does
	// not have.
	errWorkflowAliasNotFound = errors.New("workflow alias not found")
)

// Workflow is a workflow's record: what the definitions API keeps outside its
// revisions. A field the API leaves out reads "".
type Workflow struct {
	Id          string `json:"id"`
	Key         string `json:"key,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path,omitempty"`
	Nrn         string `json:"nrn,omitempty"`
}

// WorkflowRevision is one saved definition, numbered by the API.
type WorkflowRevision struct {
	Revision   int             `json:"revision"`
	Definition json.RawMessage `json:"definition"`
}

// WorkflowWithRevision is what the definitions API answers about a workflow: its
// record and one of its revisions (the latest, unless another was asked for).
type WorkflowWithRevision struct {
	Workflow Workflow         `json:"workflow"`
	Revision WorkflowRevision `json:"revision"`
}

// WorkflowTriggerState is what an alias keeps about one of its triggers since
// an activation. It is not cleared by a deactivation: it says what was
// registered once, not what is now.
type WorkflowTriggerState struct {
	RuntimeMetadata struct {
		WebhookUrl string `json:"webhookUrl,omitempty"`
	} `json:"runtimeMetadata"`
}

// WorkflowAlias is a named pointer to a revision. Active means its triggers
// were registered by an activation that succeeded.
type WorkflowAlias struct {
	WorkflowId    string                          `json:"workflowId"`
	Name          string                          `json:"name"`
	Revision      int                             `json:"revision"`
	Active        bool                            `json:"active"`
	TriggerStates map[string]WorkflowTriggerState `json:"triggerStates,omitempty"`
}

// WorkflowTriggerResult is the outcome of one trigger in an activation.
type WorkflowTriggerResult struct {
	Id         string `json:"id"`
	PluginType string `json:"pluginType"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// WorkflowActivation is the answer to an activation. The API answers 200 also
// when it failed: a failed trigger, an alias left inactive or an
// activationError say so.
type WorkflowActivation struct {
	Alias           WorkflowAlias           `json:"alias"`
	FailedTriggers  []WorkflowTriggerResult `json:"failedTriggers"`
	ActivationError string                  `json:"activationError,omitempty"`
}

// WorkflowDeactivation is the answer to a deactivation. A deactivateError is a
// release that failed: what it names is still registered.
type WorkflowDeactivation struct {
	Alias           WorkflowAlias `json:"alias"`
	DeactivateError string        `json:"deactivateError,omitempty"`
}

// WorkflowValidationError is one error of a definition the API refused.
type WorkflowValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// WorkflowError is a request the workflows API refused. Type is the problem
// document's type (a URL ending in a slug such as alias-active), "" when the
// body was not one; Message is its title and detail, the message of the
// API's other error shape, or the body itself (a gateway's page).
type WorkflowError struct {
	Op         string
	StatusCode int
	Type       string
	Message    string
	Errors     []WorkflowValidationError
}

func (e *WorkflowError) Error() string {
	text := e.Message
	for _, v := range e.Errors {
		text += fmt.Sprintf("; %s: %s", v.Path, v.Message)
		if v.Code != "" {
			text += " (" + v.Code + ")"
		}
	}
	// Redacted here, the one place every refusal is printed: a gateway may echo
	// the request.
	return redactMessage(fmt.Sprintf("error %s, got %d: %s", e.Op, e.StatusCode, text))
}

// HasType reports whether the refusal's problem type is the API's slug.
func (e *WorkflowError) HasType(slug string) bool {
	return strings.HasSuffix(e.Type, "/errors/"+slug)
}

// Is reads as not found only the API's own 404s, by their type: any other 404
// (a gateway's page) is an error, never a workflow gone.
func (e *WorkflowError) Is(target error) bool {
	switch target {
	case errWorkflowNotFound:
		return e.StatusCode == http.StatusNotFound && e.HasType("workflow-not-found")
	case errWorkflowAliasNotFound:
		return e.StatusCode == http.StatusNotFound && e.HasType("alias-not-found")
	}
	return false
}

// isWorkflowGone is a workflow or one of its aliases that the API no longer
// has.
func isWorkflowGone(err error) bool {
	return errors.Is(err, errWorkflowNotFound) || errors.Is(err, errWorkflowAliasNotFound)
}

// newWorkflowError reads a refusal's body: a problem document, the API's
// {"error": {code, message}}, or anything else as text.
func newWorkflowError(op string, status int, raw []byte) *WorkflowError {
	refused := &WorkflowError{Op: op, StatusCode: status, Message: strings.TrimSpace(string(raw))}
	var doc struct {
		Type   string                    `json:"type"`
		Title  string                    `json:"title"`
		Detail string                    `json:"detail"`
		Errors []WorkflowValidationError `json:"errors"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return refused
	}
	switch {
	case doc.Error != nil && doc.Error.Message != "":
		refused.Message = doc.Error.Message
		if doc.Error.Code != "" {
			refused.Message += " (" + doc.Error.Code + ")"
		}
	case doc.Title != "" || doc.Detail != "":
		refused.Type, refused.Errors = doc.Type, doc.Errors
		refused.Message = doc.Title
		if doc.Detail != "" && doc.Title != "" {
			refused.Message += ": "
		}
		refused.Message += doc.Detail
	}
	return refused
}

// workflowRequest sends one request and decodes the answer into out (nil: no
// body), or returns the refusal.
func (c *NullClient) workflowRequest(op, method, path string, in any, want int, out any) error {
	var body *bytes.Buffer
	if in != nil {
		body = &bytes.Buffer{}
		enc := json.NewEncoder(body)
		// The definition goes as written: "&" or "<" in a URL or an expression
		// stay readable in the request and in the logs.
		enc.SetEscapeHTML(false)
		if err := enc.Encode(in); err != nil {
			return err
		}
	}
	res, err := c.MakeRequest(method, path, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return newWorkflowError(op, res.StatusCode, raw)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func workflowPath(id string) string {
	return WORKFLOW_DEFINITIONS_PATH + "/" + url.PathEscape(id)
}

func workflowAliasPath(id, alias string) string {
	return workflowPath(id) + "/aliases/" + url.PathEscape(alias)
}

// CreateWorkflow creates a workflow and its revision 1 from body: the
// definition's keys with the record's (name, and key, description, path, nrn
// when set) beside them.
func (c *NullClient) CreateWorkflow(body map[string]json.RawMessage) (*WorkflowWithRevision, error) {
	created := &WorkflowWithRevision{}
	err := c.workflowRequest("creating workflow", http.MethodPost, WORKFLOW_DEFINITIONS_PATH, body, http.StatusCreated, created)
	return created, err
}

// GetWorkflow reads a workflow by its id or its key, with its latest revision.
func (c *NullClient) GetWorkflow(ref string) (*WorkflowWithRevision, error) {
	read := &WorkflowWithRevision{}
	err := c.workflowRequest("reading workflow "+ref, http.MethodGet, workflowPath(ref), nil, http.StatusOK, read)
	return read, err
}

// GetWorkflowRevision reads a workflow with one of its revisions.
func (c *NullClient) GetWorkflowRevision(id string, revision int) (*WorkflowWithRevision, error) {
	read := &WorkflowWithRevision{}
	err := c.workflowRequest(fmt.Sprintf("reading revision %d of workflow %s", revision, id), http.MethodGet,
		workflowPath(id)+"?revision="+strconv.Itoa(revision), nil, http.StatusOK, read)
	return read, err
}

// PutWorkflowRevision saves definition as a new revision, always: never an
// overwrite of the latest one.
func (c *NullClient) PutWorkflowRevision(id string, definition map[string]json.RawMessage) (*WorkflowWithRevision, error) {
	saved := &WorkflowWithRevision{}
	err := c.workflowRequest("saving a revision of workflow "+id, http.MethodPut, workflowPath(id),
		map[string]any{"definition": definition, "mode": "new-revision"}, http.StatusOK, saved)
	return saved, err
}

// PatchWorkflow changes the record's fields in patch; it saves no revision.
func (c *NullClient) PatchWorkflow(id string, patch map[string]any) (*Workflow, error) {
	var patched struct {
		Workflow Workflow `json:"workflow"`
	}
	err := c.workflowRequest("updating workflow "+id, http.MethodPatch, workflowPath(id), patch, http.StatusOK, &patched)
	return &patched.Workflow, err
}

// DeleteWorkflow deletes a workflow with its revisions and inactive aliases.
func (c *NullClient) DeleteWorkflow(id string) error {
	return c.workflowRequest("deleting workflow "+id, http.MethodDelete, workflowPath(id), nil, http.StatusNoContent, nil)
}

// CreateWorkflowAlias creates an inactive alias pointing at revision.
func (c *NullClient) CreateWorkflowAlias(id, name string, revision int) (*WorkflowAlias, error) {
	alias := &WorkflowAlias{}
	err := c.workflowRequest("creating alias "+name+" of workflow "+id, http.MethodPost, workflowPath(id)+"/aliases",
		map[string]any{"name": name, "revision": revision}, http.StatusCreated, alias)
	return alias, err
}

// GetWorkflowAlias reads an alias of a workflow given by its id or its key.
func (c *NullClient) GetWorkflowAlias(ref, name string) (*WorkflowAlias, error) {
	alias := &WorkflowAlias{}
	err := c.workflowRequest("reading alias "+name+" of workflow "+ref, http.MethodGet, workflowAliasPath(ref, name), nil,
		http.StatusOK, alias)
	return alias, err
}

// MoveWorkflowAlias points an alias at revision. On an active alias it
// registers nothing: the new revision's triggers need an activation.
func (c *NullClient) MoveWorkflowAlias(id, name string, revision int) (*WorkflowAlias, error) {
	alias := &WorkflowAlias{}
	err := c.workflowRequest(fmt.Sprintf("moving alias %s of workflow %s to revision %d", name, id, revision),
		http.MethodPut, workflowAliasPath(id, name), map[string]any{"revision": revision}, http.StatusOK, alias)
	return alias, err
}

// ActivateWorkflowAlias registers the triggers of the alias's revision. It
// sends no body: the activation is of the revision the alias points at.
func (c *NullClient) ActivateWorkflowAlias(id, name string) (*WorkflowActivation, error) {
	activation := &WorkflowActivation{}
	err := c.workflowRequest("activating alias "+name+" of workflow "+id, http.MethodPost,
		workflowAliasPath(id, name)+"/activate", nil, http.StatusOK, activation)
	return activation, err
}

// DeactivateWorkflowAlias marks an alias inactive and releases its triggers,
// from what the API stored about them.
func (c *NullClient) DeactivateWorkflowAlias(id, name string) (*WorkflowDeactivation, error) {
	deactivation := &WorkflowDeactivation{}
	err := c.workflowRequest("deactivating alias "+name+" of workflow "+id, http.MethodPost,
		workflowAliasPath(id, name)+"/deactivate", nil, http.StatusOK, deactivation)
	return deactivation, err
}

// DeleteWorkflowAlias deletes an alias.
func (c *NullClient) DeleteWorkflowAlias(id, name string) error {
	return c.workflowRequest("deleting alias "+name+" of workflow "+id, http.MethodDelete, workflowAliasPath(id, name), nil,
		http.StatusNoContent, nil)
}

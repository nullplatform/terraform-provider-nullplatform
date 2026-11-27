package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const APPLICATION_PATH = "/application"

type Application struct {
	Id                   int                    `json:"id,omitempty"`
	Name                 string                 `json:"name,omitempty"`
	Status               string                 `json:"status,omitempty"`
	NamespaceId          int                    `json:"namespace_id,omitempty"`
	RepositoryUrl        string                 `json:"repository_url,omitempty"`
	Slug                 string                 `json:"slug,omitempty"`
	TemplateId           int                    `json:"template_id,omitempty"`
	AutoDeployOnCreation bool                   `json:"auto_deploy_on_creation,omitempty"`
	RepositoryAppPath    string                 `json:"repository_app_path,omitempty"`
	IsMonoRepo           bool                   `json:"is_mono_repo,omitempty"`
	Tags                 map[string]interface{} `json:"tags,omitempty"`
	Settings             map[string]interface{} `json:"settings,omitempty"`
	Messages             []interface{}          `json:"messages,omitempty"`
	Nrn                  string                 `json:"nrn,omitempty"`
	// The type of the application, the snapshot of it the application is on,
	// and the values of the type's fields. All three stay out of a request
	// that does not set them.
	ApplicationTypeId         string `json:"application_type_id,omitempty"`
	ApplicationTypeSnapshotId string `json:"application_type_snapshot_id,omitempty"`
	// The snapshot of its type the application is asked to move to. The
	// platform moves it with the edit that carries it, once the attributes pass
	// that snapshot's schema, and reads it back empty.
	DesiredApplicationTypeSnapshotId string `json:"desired_application_type_snapshot_id,omitempty"`
	// Left out when nil. An empty object is sent: it is the edit that changes
	// no attribute, which a move to another snapshot goes with.
	Attributes map[string]interface{} `json:"attributes,omitzero"`
}

func (c *NullClient) CreateApplication(application *Application) (*Application, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*application); err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", APPLICATION_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("error creating application resource, got status code: %d, message: %s", res.StatusCode, applicationRefusal(res))
	}

	app := &Application{}
	if err := json.NewDecoder(res.Body).Decode(app); err != nil {
		return nil, err
	}

	return app, nil
}

func (c *NullClient) PatchApplication(appId string, application *Application) error {
	path := fmt.Sprintf("%s/%s", APPLICATION_PATH, appId)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*application); err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("error updating application resource, got status code: %d, message: %s", res.StatusCode, applicationRefusal(res))
	}

	return nil
}

func (c *NullClient) DeleteApplication(appId string) error {
	path := fmt.Sprintf("%s/%s", APPLICATION_PATH, appId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("error making DELETE request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("error deleting application, got status code: %d, message: %s", res.StatusCode, applicationRefusal(res))
	}

	return nil
}

func (c *NullClient) GetApplication(appId string) (*Application, error) {
	path := fmt.Sprintf("%s/%s", APPLICATION_PATH, appId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Check the status before decoding: an error body carries a numeric
	// "status" that does not fit Application.Status, which used to hide the
	// real response (e.g. a 404 for an application deleted outside Terraform).
	if res.StatusCode != http.StatusOK {
		message := applicationRefusal(res)
		if res.StatusCode == http.StatusNotFound {
			id, _ := strconv.Atoi(appId)
			return nil, &ResourceNotFoundError{ApiType: "application", ID: id, Message: message}
		}
		return nil, fmt.Errorf("error getting application resource, got %d for %s: %s", res.StatusCode, appId, message)
	}

	app := &Application{}
	if err := json.NewDecoder(res.Body).Decode(app); err != nil {
		return nil, err
	}

	return app, nil
}

// applicationRefusal is what the platform answered a refused application
// request with: the error's message, followed by its `detail` when the
// platform says more there. The platform's message is the error's static
// description ("The application type cannot be used for this application");
// what it refused (the type, the namespace, the attribute) is in `detail`.
// Only those two members are read: the platform numbers its errors with a
// UUID `id`, which NullErrors cannot hold. A body with no message (a
// gateway's page) is the message as it came.
func applicationRefusal(res *http.Response) string {
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var refusal struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(raw, &refusal) != nil || refusal.Message == "" {
		return redactMessage(strings.TrimSpace(string(raw)))
	}
	message := refusal.Message
	if refusal.Detail != "" && refusal.Detail != refusal.Message {
		message += ": " + refusal.Detail
	}
	return redactMessage(message)
}

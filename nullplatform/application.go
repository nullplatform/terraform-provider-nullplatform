package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode null error response: %w", err)
		}
		return nil, fmt.Errorf("error creating application resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
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
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode null error response: %w", err)
		}
		return fmt.Errorf("error updating application resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
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
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		return fmt.Errorf("error deleting application, got status code: %d, message: %s", res.StatusCode, nErr.Message)
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

	app := &Application{}
	derr := json.NewDecoder(res.Body).Decode(app)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting application resource, got %d for %s", res.StatusCode, appId)
	}

	return app, nil
}

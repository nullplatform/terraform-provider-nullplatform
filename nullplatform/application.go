package nullplatform

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const APPLICATION_PATH = "/application"

type Application struct {
	Id                   int    `json:"id,omitempty"`
	Name                 string `json:"name,omitempty"`
	Status               string `json:"status,omitempty"`
	NamespaceId          int    `json:"namespace_id,omitempty"`
	RepositoryUrl        string `json:"repository_url,omitempty"`
	Slug                 string `json:"slug,omitempty"`
	TemplateId           int    `json:"template_id,omitempty"`
	AutoDeployOnCreation bool   `json:"auto_deploy_on_creation,omitempty"`
	RepositoryAppPath    string `json:"repository_app_path,omitempty"`
	IsMonoRepo           bool   `json:"is_mono_repo,omitempty"`
	Nrn                  string `json:"nrn,omitempty"`
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
		return nil, fmt.Errorf("Error getting application resource, got %d for %s", res.StatusCode, appId)
	}

	return app, nil
}

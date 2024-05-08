package nullplatform

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, APPLICATION_PATH, appId)

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

	app := &Application{}
	derr := json.NewDecoder(res.Body).Decode(app)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(os.Stdout, res.Body)
		return nil, fmt.Errorf("error getting application resource, got %d for %s", res.StatusCode, appId)
	}

	return app, nil
}

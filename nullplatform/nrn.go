package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const NRN_PATH = "/nrn"

type PatchNRN struct {
	AWSS3AssestBucket               string `json:"aws.s3_assets_bucket"`
	AWSScopeWorkflowRole            string `json:"aws.scope_workflow_role"`
	AWSLogGroupName                 string `json:"aws.log_group_name"`
	AWSLambdaFunctionName           string `json:"aws.lambdaFunctionName"`
	AWSLambdaCurrentFunctionVersion string `json:"aws.lambdaCurrentFunctionVersion"`
	AWSLambdaFunctionRole           string `json:"aws.lambdaFunctionRole"`
	AWSLambdaFunctionMainAlias      string `json:"aws.lambdaFunctionMainAlias"`
	AWSLogReaderLog                 string `json:"aws.log_reader_role"`
	AWSLambdaFunctionWarmAlias      string `json:"aws.lambdaFunctionWarmAlias"`
}

type Namespaces struct {
	Aws    map[string]string `json:"aws"`
	Github map[string]string `json:"github"`
	Global map[string]string `json:"global"`
}
type NRN struct {
	Nrn        string            `json:"nrn"`
	Namespaces map[string]string `json:"namespaces"`
}

func (c *NullClient) PatchNRN(nrnId string, nrn *PatchNRN) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode((*nrn))

	if err != nil {
		return err
	}

	r, err := http.NewRequest("PATCH", fmt.Sprintf("htpps://%s%s/%s", c.ApiURL, NRN_PATH, nrnId), &buf)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	resp, err := c.Client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error creating resource, got %d", resp.StatusCode)
	}

	return nil
}

func (c *NullClient) GetNRN(nrnId string) (*NRN, error) {
	// It is not ideal to harcode ids=aws.* but for now will retrieve everything we need
	url := fmt.Sprintf("https://%s%s/%s?ids=aws.*", c.ApiURL, NRN_PATH, nrnId)

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

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error creating resource, got %d", res.StatusCode)
	}

	s := &NRN{}
	derr := json.NewDecoder(res.Body).Decode(s)

	if derr != nil {
		return nil, derr
	}

	return s, nil

}

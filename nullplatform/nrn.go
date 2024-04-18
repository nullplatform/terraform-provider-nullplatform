package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const NRN_PATH = "/nrn"

// PathNRN does not have the `omitempty` to be able to keep values empty
type PatchNRN struct {
	AWSS3AssestBucket               string `json:"aws.s3_assets_bucket"`
	AWSScopeWorkflowRole            string `json:"aws.scope_workflow_role"`
	AWSLogGroupName                 string `json:"aws.log_group_name"`
	AWSLambdaFunctionName           string `json:"aws.lambdaFunctionName,omitempty"`
	AWSLambdaCurrentFunctionVersion string `json:"aws.lambdaCurrentFunctionVersion,omitempty"`
	AWSLambdaFunctionRole           string `json:"aws.lambdaFunctionRole,omitempty"`
	AWSLambdaFunctionMainAlias      string `json:"aws.lambdaFunctionMainAlias,omitempty"`
	AWSLogReaderLog                 string `json:"aws.log_reader_role"`
	AWSLambdaFunctionWarmAlias      string `json:"aws.lambdaFunctionWarmAlias"`
}

// Same structure as PatchNRN but to read
type NrnAwsNamespace struct {
	AWSS3AssestBucket               string `json:"s3_assets_bucket,omitempty"`
	AWSScopeWorkflowRole            string `json:"scope_workflow_role,omitempty"`
	AWSLogGroupName                 string `json:"log_group_name,omitempty"`
	AWSLambdaFunctionName           string `json:"lambdaFunctionName,omitempty"`
	AWSLambdaCurrentFunctionVersion string `json:"lambdaCurrentFunctionVersion,omitempty"`
	AWSLambdaFunctionRole           string `json:"lambdaFunctionRole,omitempty"`
	AWSLambdaFunctionMainAlias      string `json:"lambdaFunctionMainAlias,omitempty"`
	AWSLogReaderLog                 string `json:"log_reader_role,omitempty"`
	AWSLambdaFunctionWarmAlias      string `json:"lambdaFunctionWarmAlias,omitempty"`
}

type Namespaces struct {
	AWS    *NrnAwsNamespace  `json:"aws,omitempty"`
	Github map[string]string `json:"github,omitempty"`
	Global map[string]string `json:"global,omitempty"`
}

type NRN struct {
	Nrn        string      `json:"nrn,omitempty"`
	Namespaces *Namespaces `json:"namespaces,omitempty"`
	//Profiles   map[string]map[string]string `json:"profiles,omitempty"`
}

func (c *NullClient) PatchNRN(nrnId string, nrn *PatchNRN) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode((*nrn))

	if err != nil {
		return err
	}

	r, err := http.NewRequest("PATCH", fmt.Sprintf("https://%s%s/%s", c.ApiURL, NRN_PATH, nrnId), &buf)
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, bytes.NewReader(buf.Bytes()))

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		io.Copy(os.Stdout, res.Body)
		return fmt.Errorf("error patching nrn resource, got %d: %+v", res.StatusCode, res.Body)
	}

	return nil
}

func (c *NullClient) GetNRN(nrnId string) (*NRN, error) {
	namespaces := []string{
		"aws.scope_workflow_role",
		"aws.log_group_name",
		"aws.lambdaFunctionName",
		"aws.lambdaCurrentFunctionVersion",
		"aws.lambdaFunctionRole",
		"aws.lambdaFunctionMainAlias",
		"aws.log_reader_role",
		"aws.lambdaFunctionWarmAlias",
	}
	url := fmt.Sprintf("https://%s%s/%s?ids=%s", c.ApiURL, NRN_PATH, nrnId, strings.Join(namespaces, ","))

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
		io.Copy(os.Stdout, res.Body)
		return nil, fmt.Errorf("error getting nrn resource, got %d: %+v", res.StatusCode, res.Body)
	}

	s := &NRN{}
	derr := json.NewDecoder(res.Body).Decode(s)

	if derr != nil {
		return nil, derr
	}

	return s, nil

}

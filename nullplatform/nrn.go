package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

const NRN_PATH = "/nrn"

// Vales with `omitempty` shoudn't be patched as empty values, the others can.
// The struct is used to PATCH the NRN
type PatchNRN struct {
	AWSS3AssestBucket               string `json:"aws.s3_assets_bucket,omitempty"`
	AWSScopeWorkflowRole            string `json:"aws.scope_workflow_role,omitempty"`
	AWSLogGroupName                 string `json:"aws.log_group_name,omitempty"`
	AWSLambdaFunctionName           string `json:"aws.lambdaFunctionName,omitempty"`
	AWSLambdaCurrentFunctionVersion string `json:"aws.lambdaCurrentFunctionVersion,omitempty"`
	AWSLambdaFunctionRole           string `json:"aws.lambdaFunctionRole,omitempty"`
	AWSLambdaFunctionMainAlias      string `json:"aws.lambdaFunctionMainAlias,omitempty"`
	AWSLogReaderLog                 string `json:"aws.log_reader_role,omitempty"`
	AWSLambdaFunctionWarmAlias      string `json:"aws.lambdaFunctionWarmAlias,omitempty"`
}

// Similar structure to PatchNRN but without the `.aws`.
// The struct is used to READ the NRN
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
}

func (c *NullClient) PatchNRN(nrnId string, nrn *PatchNRN) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode((*nrn))
	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", fmt.Sprintf("%s/%s", NRN_PATH, nrnId), &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error patching nrn resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) GetNRN(nrnId string) (*NRN, error) {
	// Slice to store JSON attributes
	var namespaces []string

	// Using `aws.*` returns an error, so instead
	// use reflection to obtain the JSON attributes for the struct
	patchNRN := PatchNRN{}
	t := reflect.TypeOf(patchNRN)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		namespaces = append(namespaces, jsonTag)
	}

	path := fmt.Sprintf("%s/%s?ids=%s", NRN_PATH, nrnId, strings.Join(namespaces, ","))

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting nrn resource, got %d", res.StatusCode)
	}

	s := &NRN{}
	derr := json.NewDecoder(res.Body).Decode(s)

	if derr != nil {
		return nil, derr
	}

	return s, nil

}

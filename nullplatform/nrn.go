package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const NRN_PATH = "/nrn"

// Vales with `omitempty` shoudn't be patched as empty values, the others can.
// The struct is used to PATCH the NRN
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

type NRNComponent struct {
	Key       string
	GetFunc   func(*NullClient, string, string) (map[string]interface{}, error)
	ParentKey string
}

var NRNComponents = []NRNComponent{
	{"account", (*NullClient).GetAccountBySlug, "organization"},
	{"namespace", (*NullClient).GetNamespaceBySlug, "account"},
	{"application", (*NullClient).GetApplicationBySlug, "namespace"},
	{"scope", (*NullClient).GetScopeBySlug, "application"},
}

var NRNSchema = map[string]*schema.Schema{
	"account": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The account component of the NRN",
	},
	"namespace": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The namespace component of the NRN",
	},
	"application": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The application component of the NRN",
	},
	"scope": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The scope component of the NRN",
	},
	"nrn": {
		Type:          schema.TypeString,
		Optional:      true,
		Description:   "A system-wide unique ID representing the resource.",
		ConflictsWith: []string{"account", "namespace", "application", "scope"},
	},
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

func AddNRNSchema(s map[string]*schema.Schema) map[string]*schema.Schema {
	for k, v := range NRNSchema {
		s[k] = v
	}
	return s
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

func ConstructNRNFromComponents(d *schema.ResourceData, nullOps NullOps) (string, error) {
	client, ok := nullOps.(*NullClient)
	if !ok {
		return "", fmt.Errorf("error asserting NullClient")
	}

	organizationID, err := client.GetOrganizationIDFromToken()
	if err != nil {
		return "", fmt.Errorf("error getting organization ID from token: %v", err)
	}

	nrnParts := []string{fmt.Sprintf("organization=%s", organizationID)}

	parentID := organizationID
	for _, component := range NRNComponents {
		if v, ok := d.GetOk(component.Key); ok {
			key := component.Key

			result, err := component.GetFunc(client, parentID, v.(string))
			if err != nil {
				return "", fmt.Errorf("error resolving %s: %v", key, err)
			}

			id, ok := result["id"].(string)
			if !ok || id == "" {
				return "", fmt.Errorf("%s not found or invalid ID: %s", key, v.(string))
			}

			nrnParts = append(nrnParts, fmt.Sprintf("%s=%s", key, id))
			parentID = id
		} else {
			break
		}
	}

	return strings.Join(nrnParts, ":"), nil
}

func (c *NullClient) GetAccountBySlug(organizationID, slug string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/account?organization_id=%s&slug=%s", organizationID, slug)
	return c.getEntityBySlug(path)
}

func (c *NullClient) GetNamespaceBySlug(accountID, slug string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/namespace?account_id=%s&slug=%s", accountID, slug)
	return c.getEntityBySlug(path)
}

func (c *NullClient) GetApplicationBySlug(namespaceID, slug string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/application?namespace_id=%s&slug=%s", namespaceID, slug)
	return c.getEntityBySlug(path)
}

func (c *NullClient) GetScopeBySlug(applicationID, slug string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/scope?application_id=%s&slug=%s", applicationID, slug)
	return c.getEntityBySlug(path)
}

func (c *NullClient) getEntityBySlug(path string) (map[string]interface{}, error) {
	resp, err := c.MakeRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	if len(result.Results) == 0 {
		return nil, fmt.Errorf("entity not found")
	}

	return result.Results[0], nil
}

package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceScope() *schema.Resource {
	return &schema.Resource{
		Create: ScopeCreate,
		Read:   ScopeRead,
		Update: ScopeUpdate,
		Delete: ScopeDelete,

		Schema: map[string]*schema.Schema{
			"last_updated": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"scope_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"null_application_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"s3_assets_bucket": {
				Type:     schema.TypeString,
				Required: true,
			},
			"scope_workflow_role": {
				Type:     schema.TypeString,
				Required: true,
			},
			"log_group_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"lambda_function_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"lambda_current_function_version": {
				Type:     schema.TypeString,
				Required: true,
			},
			"lambda_function_role": {
				Type:     schema.TypeString,
				Required: true,
			},
			"lambda_function_main_alias": {
				Type:     schema.TypeString,
				Required: true,
			},
			"log_reader_role": {
				Type:     schema.TypeString,
				Required: true,
			},
			"lambda_function_warm_alias": {
				Type:     schema.TypeString,
				Required: true,
			},
			"capabilities_serverless_handler_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"capabilities_serverless_runtime_id": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func ScopeCreate(d *schema.ResourceData, m any) error {
	url := "https://api.nullplatform.com/scope" //Content-Type: application/json" -H"Authorization: Bearer $NULL_TOKEN" -d "$post_data"
	accessToken := m.(string)

	log.Print("\n\n--- CREATE Serverless scope ---\n\n")
	log.Printf("\n\n>>> schema.ResourceData: %+v\n\n", d)
	log.Printf("\n\n>>> meta data: %+v\n\n", m)

	scopeName := d.Get("scope_name").(string)
	applicationId := d.Get("null_application_id").(int)
	serverless_runtime := d.Get("capabilities_serverless_runtime_id").(string)
	serverless_handler := d.Get("capabilities_serverless_handler_name").(string)
	scopeType := "serverless"

	// c := m.(*some.APIClient)// Create a HTTP post request
	newScope := ScopeReq{
		Name:            scopeName,
		ApplicationId:   applicationId,
		Type:            scopeType,
		ExternalCreated: true,
		RequestedSpec: RequestSpec{
			MemoryInGb:   0.5,
			CpuProfile:   "standard",
			LocalStorage: 8,
		},
		Capabilities: Capability{
			Visibility: map[string]string{"reachability": "account"},
			ServerlessRuntime: map[string]string{
				"serverless_runtime": "account",
				"id":                 serverless_runtime,
			},
			ServerlessHandler:          map[string]string{"name": serverless_handler},
			ServerlessTimeout:          map[string]int{"timeout_in_seconds": 3},
			ServerlessEphemeralStorage: map[string]int{"memory_in_mb": 512},
			ServerlessMemory:           map[string]int{"memory_in_mb": 128},
		},
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(newScope)

	if err != nil {
		return err
	}

	client := &http.Client{}
	r, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	res, err := client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	rcsResp := &ScopeResponse{}
	derr := json.NewDecoder(res.Body).Decode(rcsResp)

	if derr != nil {
		return derr
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("error creating resource, got %d", res.StatusCode)
	}

	log.Print("\n\n--- BEFORE patch NRN ---\n\n")

	nrnErr := createNrnForScope(rcsResp.Nrn, accessToken, d, m)

	if nrnErr != nil {
		log.Print("\n\n--- AFTER patch NRN failed ******---\n\n")
		return nrnErr
	}

	log.Print("\n\n--- AFTER patch NRN success ---\n\n")

	d.SetId(strconv.Itoa(rcsResp.Id))

	return ScopeRead(d, m)
}

func createNrnForScope(scopeNrn string, accessToken string, d *schema.ResourceData, _ any) error {
	url := fmt.Sprintf("https://api.nullplatform.com/nrn/%s", scopeNrn)

	s3AssetsBucket := d.Get("s3_assets_bucket").(string)
	scopeWorkflowRole := d.Get("scope_workflow_role").(string)
	logGroupName := d.Get("log_group_name").(string)
	lambdaFunctinoName := d.Get("lambda_function_name").(string)
	lambdaCurrentFunctionVersion := d.Get("lambda_current_function_version").(string)
	lambdaFunctionRole := d.Get("lambda_function_role").(string)
	lambdaFunctionMainAlias := d.Get("lambda_function_main_alias").(string)
	logReaderRole := d.Get("log_reader_role").(string)
	lambdaFunctionWarmAlias := d.Get("lambda_function_warm_alias").(string)

	// c := m.(*some.APIClient)// Create a HTTP post request
	patchResource := Resource{
		AWSS3AssestBucket:               s3AssetsBucket,
		AWSScopeWorkflowRole:            scopeWorkflowRole,
		AWSLogGroupName:                 logGroupName,
		AWSLambdaFunctionName:           lambdaFunctinoName,
		AWSLambdaCurrentFunctionVersion: lambdaCurrentFunctionVersion,
		AWSLambdaFunctionRole:           lambdaFunctionRole,
		AWSLambdaFunctionMainAlias:      lambdaFunctionMainAlias,
		AWSLogReaderLog:                 logReaderRole,
		AWSLambdaFunctionWarmAlias:      lambdaFunctionWarmAlias,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(patchResource)

	if err != nil {
		return err
	}

	client := &http.Client{}
	r, err := http.NewRequest("PATCH", url, &buf)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error creating resource, got %d", resp.StatusCode)
	}

	return nil
}

func ScopeRead(d *schema.ResourceData, m any) error {
	resourceID := d.Id()

	url := fmt.Sprintf("https://api.nullplatform.com/scope/%s", resourceID)
	accessToken := m.(string)

	client := &http.Client{}
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	res, err := client.Do(r)
	if err != nil {
		d.SetId("")
		return err
	}
	defer res.Body.Close()

	rcsResp := &ScopeResponse{}
	derr := json.NewDecoder(res.Body).Decode(rcsResp)

	if derr != nil {
		d.SetId("")
		return derr
	}

	if res.StatusCode != http.StatusOK {
		d.SetId("")
		return fmt.Errorf("error getting resource, got %d for %s", res.StatusCode, resourceID)
	}

	log.Print("\n\n--- READ ---\n\n")
	log.Printf("\n\n>>> schema.ResourceData: %+v\n\n", d)
	log.Printf("\n\n>>> meta data: %+v\n\n", m)

	// I want to set a computed value for the nested 'version' attribute, but to
	// do that I have to iterate over each parent structure until I reach the
	// relevant level of the data structure where I can then set a value on the
	// 'version' attribute.
	if err := d.Set("scope_name", rcsResp.Name); err != nil {
		return err
	}
	if err := d.Set("null_application_id", rcsResp.ApplicationId); err != nil {
		return err
	}
	// I also make sure to update the computed 'last_update' attribute every time
	// we update the terraform state.
	d.Set("last_updated", time.Now().Format(time.RFC850))

	return nil
}

func getNrnForScope(scopeNrn string, accessToken string, d *schema.ResourceData, _ any) error {
	url := fmt.Sprintf("https://api.nullplatform.com/nrn/%s", scopeNrn)

	s3AssetsBucket := d.Get("s3_assets_bucket").(string)
	scopeWorkflowRole := d.Get("scope_workflow_role").(string)
	logGroupName := d.Get("log_group_name").(string)
	lambdaFunctinoName := d.Get("lambdaFunctionName").(string)
	lambdaCurrentFunctionVersion := d.Get("lambdaCurrentFunctionVersion").(string)
	lambdaFunctionRole := d.Get("lambdaFunctionRole").(string)
	lambdaFunctionMainAlias := d.Get("lambdaFunctionMainAlias").(string)
	logReaderRole := d.Get("log_reader_role").(string)
	lambdaFunctionWarmAlias := d.Get("lambdaFunctionWarmAlias").(string)

	// c := m.(*some.APIClient)// Create a HTTP post request
	patchResource := Resource{
		AWSS3AssestBucket:               s3AssetsBucket,
		AWSScopeWorkflowRole:            scopeWorkflowRole,
		AWSLogGroupName:                 logGroupName,
		AWSLambdaFunctionName:           lambdaFunctinoName,
		AWSLambdaCurrentFunctionVersion: lambdaCurrentFunctionVersion,
		AWSLambdaFunctionRole:           lambdaFunctionRole,
		AWSLambdaFunctionMainAlias:      lambdaFunctionMainAlias,
		AWSLogReaderLog:                 logReaderRole,
		AWSLambdaFunctionWarmAlias:      lambdaFunctionWarmAlias,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(patchResource)

	if err != nil {
		return err
	}

	client := &http.Client{}
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error creating resource, got %d", resp.StatusCode)
	}

	return nil
}

func ScopeUpdate(d *schema.ResourceData, m any) error {
	log.Print("\n\n--- UPDATE ---\n\n")
	log.Printf("\n\n>>> schema.ResourceData: %+v\n\n", d)
	log.Printf("\n\n>>> meta data: %+v\n\n", m)

	// We get the ID we set into terraform state after we had initially created
	// the resource.
	resourceID := d.Id()
	log.Println("resourceID:", resourceID)

	if d.HasChange("foo") {
		foo := d.Get("foo").([]any)
		log.Printf(">>> foo: %+v\n", foo)

		// Imagine we made an API call to update the given resource.
		//
		// We'd do this by iterating over the foo we pulled out of our terraform
		// state and coercing them into a type of map[string]any
		//
		// e.g.
		//
		// for _, f := range foo {
		// 	i := f.(map[string]any)
		//
		// 	t := i["bar"].([]any)[0]
		// 	bar := t.(map[string]any)
		//
		//  ...constructing data structure to pass to API...
		//
		//  We might assign values to the data structure like:
		//
		//  bar["id"].(int)).
		// }

		// TODO: update "version" to be 2

		d.Set("last_updated", time.Now().Format(time.RFC850))
	}

	// Again, we do a READ operation to be sure we get the latest state stored locally.
	//
	return ScopeRead(d, m)
}

func ScopeDelete(d *schema.ResourceData, m any) error {
	log.Print("\n\n--- DELETE ---\n\n")
	log.Printf("\n\n>>> schema.ResourceData: %+v\n\n", d)
	log.Printf("\n\n>>> meta data: %+v\n\n", m)

	// We get the ID we set into terraform state after we had initially created
	// the resource.
	resourceID := d.Id()
	log.Println(">>> resourceID:", resourceID)

	// Imagine we use resourceID to issue a DELETE API call.

	// d.SetId("") is automatically called assuming delete returns no errors, but
	// it is added here for explicitness.
	d.SetId("")

	return nil
}

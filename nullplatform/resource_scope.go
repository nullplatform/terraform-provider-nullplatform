package nullplatform

import (
	"log"
	"reflect"
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
	nullOps := m.(NullOps)

	log.Print("--- CREATE Serverless scope ---")
	log.Printf(">>> schema.ResourceData: %+v", d)
	log.Printf(">>> meta data: %+v", m)

	scopeName := d.Get("scope_name").(string)
	applicationId := d.Get("null_application_id").(int)
	serverless_runtime := d.Get("capabilities_serverless_runtime_id").(string)
	serverless_handler := d.Get("capabilities_serverless_handler_name").(string)
	scopeType := "serverless"

	newScope := &Scope{
		Name:            scopeName,
		ApplicationId:   applicationId,
		Type:            scopeType,
		ExternalCreated: true,
		RequestedSpec: &RequestSpec{
			MemoryInGb:   0.5,
			CpuProfile:   "standard",
			LocalStorage: 8,
		},
		Capabilities: &Capability{
			Visibility: map[string]string{
				"reachability": "account",
			},
			ServerlessRuntime: map[string]string{
				"provider": "aws_lambda",
				"id":       serverless_runtime,
			},
			ServerlessHandler: map[string]string{
				"name": serverless_handler,
			},
			ServerlessTimeout: map[string]int{
				"timeout_in_seconds": 3,
			},
			ServerlessEphemeralStorage: map[string]int{
				"memory_in_mb": 512,
			},
			ServerlessMemory: map[string]int{
				"memory_in_mb": 128,
			},
		},
	}

	s, err := nullOps.CreateScope(newScope)

	if err != nil {
		return err
	}

	log.Print("--- BEFORE patch NRN ---")

	nrnErr := createNrnForScope(s.Nrn, d, m)

	if nrnErr != nil {
		log.Print("--- AFTER patch NRN failed ******---")
		return nrnErr
	}

	log.Print("--- AFTER patch NRN success ---")

	d.SetId(strconv.Itoa(s.Id))

	return ScopeRead(d, m)
}

func createNrnForScope(scopeNrn string, d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	s3AssetsBucket := d.Get("s3_assets_bucket").(string)
	scopeWorkflowRole := d.Get("scope_workflow_role").(string)
	logGroupName := d.Get("log_group_name").(string)
	lambdaFunctinoName := d.Get("lambda_function_name").(string)
	lambdaCurrentFunctionVersion := d.Get("lambda_current_function_version").(string)
	lambdaFunctionRole := d.Get("lambda_function_role").(string)
	lambdaFunctionMainAlias := d.Get("lambda_function_main_alias").(string)
	logReaderRole := d.Get("log_reader_role").(string)
	lambdaFunctionWarmAlias := d.Get("lambda_function_warm_alias").(string)

	nrnReq := &PatchNRN{
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

	return nullOps.PatchNRN(scopeNrn, nrnReq)
}

func ScopeRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	scopeID := d.Id()

	log.Print("--- Terraform 'read resource Scope' operation begin ---")
	s, err := nullOps.GetScope(scopeID)

	if err != nil {
		d.SetId("")
		return err
	}

	log.Printf(">>> schema.ResourceData: %+v", d)
	log.Printf(">>> meta data: %+v", m)

	if err := d.Set("scope_name", s.Name); err != nil {
		return err
	}
	if err := d.Set("null_application_id", s.ApplicationId); err != nil {
		return err
	}

	log.Print("--- Terraform 'read resource Scope' operation ends ---")

	d.Set("last_updated", time.Now().Format(time.RFC850))

	return nil
}

func getNrnForScope(scopeNrn string, nullOps NullOps) (*NRN, error) {
	nrn, err := nullOps.GetNRN(scopeNrn)

	if err != nil {
		return nil, err
	}

	return nrn, nil
}

func ScopeUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	log.Print("--- Terraform 'update resource Scope' operation begin  ---")
	log.Printf(">>> schema.ResourceData: %+v", d)
	log.Printf(">>> meta data: %+v", m)

	scopeID := d.Id()

	log.Println("scopeID:", scopeID)

	ps := &Scope{}

	if d.HasChange("scope_name") {
		ps.Name = d.Get("scope_name").(string)
	}

	caps := &Capability{}

	if d.HasChange("capabilities_serverless_runtime_id") {
		caps.ServerlessRuntime = map[string]string{
			"id": d.Get("capabilities_serverless_runtime_id").(string),
		}
	}

	if d.HasChange("capabilities_serverless_handler_name") {
		caps.ServerlessHandler = map[string]string{
			"name": d.Get("capabilities_serverless_handler_name").(string),
		}
	}

	if !reflect.DeepEqual(caps, Capability{}) {
		ps.Capabilities = caps
	}

	log.Print("--- Scope updated ---")
	log.Printf(">>> schema.ResourceData: %+v", d)
	log.Printf(">>> meta data: %+v", m)

	d.Set("last_updated", time.Now().Format(time.RFC850))

	if !reflect.DeepEqual(*ps, Scope{}) {
		err := nullOps.PatchScope(scopeID, ps)
		if err != nil {
			return nil
		}
	}

	log.Print("--- Terraform 'update resource Scope' operation ends ---")

	return nil
}

func ScopeDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	scopeID := d.Id()

	log.Print("--- Terraform 'delete resource Scope' operation begin ---")
	log.Printf(">>> schema.ResourceData: %+v", d)
	log.Printf(">>> meta data: %+v", m)

	pScope := &Scope{
		Status: "deleting",
	}

	log.Print("--- Scope on: 'deleting' ---")
	err := nullOps.PatchScope(scopeID, pScope)
	if err != nil {
		return err
	}

	pScope.Status = "deleted"

	log.Print("--- Scope on: 'deleted' ---")

	err = nullOps.PatchScope(scopeID, pScope)
	if err != nil {
		return err
	}

	log.Printf(">>> schema.ResourceData: %+v", d)
	log.Printf(">>> meta data: %+v", m)

	log.Println(">>> scopeID:", scopeID)

	log.Print("--- Terraform 'delete resource Scope' operation ends ---")

	d.SetId("")

	return nil
}

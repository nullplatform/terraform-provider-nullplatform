package nullplatform

import (
	"context"
	"log"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceScope() *schema.Resource {
	return &schema.Resource{
		Create: ScopeCreate,
		Read:   ScopeRead,
		Update: ScopeUpdate,
		Delete: ScopeDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"scope_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"scope_type": {
				Type:     schema.TypeString,
				Default:  "serverless",
				Optional: true,
			},
			"null_application_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"s3_assets_bucket": {
				Type:     schema.TypeString,
				Default:  "",
				Optional: true,
			},
			"scope_workflow_role": {
				Type:     schema.TypeString,
				Default:  "",
				Optional: true,
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
				Default:  "",
				Optional: true,
			},
			"lambda_function_warm_alias": {
				Type:     schema.TypeString,
				Default:  "",
				Optional: true,
			},
			"capabilities_serverless_handler_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"capabilities_serverless_timeout": {
				Type:     schema.TypeInt,
				Default:  10,
				Optional: true,
			},
			"capabilities_serverless_runtime_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"capabilities_serverless_ephemeral_storage": {
				Type:     schema.TypeInt,
				Default:  512,
				Optional: true,
			},
			"capabilities_serverless_memory": {
				Type:     schema.TypeInt,
				Default:  128,
				Optional: true,
			},
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"runtime_configurations": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
		},
	}
}

func ScopeCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	applicationId := d.Get("null_application_id").(int)
	scopeName := d.Get("scope_name").(string)
	scopeType := d.Get("scope_type").(string)
	serverless_runtime := d.Get("capabilities_serverless_runtime_id").(string)
	serverless_handler := d.Get("capabilities_serverless_handler_name").(string)
	serverless_timeout := d.Get("capabilities_serverless_timeout").(int)
	serverless_ephemeral_storage := d.Get("capabilities_serverless_ephemeral_storage").(int)
	serverless_memory := d.Get("capabilities_serverless_memory").(int)

	dimensionsMap := d.Get("dimensions").(map[string]any)
	// Convert the dimensions to a map[string]string
	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

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
				"timeout_in_seconds": serverless_timeout,
			},
			ServerlessEphemeralStorage: map[string]int{
				"memory_in_mb": serverless_ephemeral_storage,
			},
			ServerlessMemory: map[string]int{
				"memory_in_mb": serverless_memory,
			},
		},
		Dimensions: dimensions,
	}

	s, err := nullOps.CreateScope(newScope)

	if err != nil {
		return err
	}

	nrnErr := patchNrnForScope(s.Nrn, d, m)

	if nrnErr != nil {
		return nrnErr
	}

	d.SetId(strconv.Itoa(s.Id))

	return ScopeRead(d, m)
}

func patchNrnForScope(scopeNrn string, d *schema.ResourceData, m any) error {
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

	if !reflect.DeepEqual(nrnReq, PatchNRN{}) {
		return nullOps.PatchNRN(scopeNrn, nrnReq)
	}

	return nil
}

func ScopeRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	scopeID := d.Id()

	s, err := nullOps.GetScope(scopeID)

	if err != nil {
		d.SetId("")
		return err
	}

	n, err := nullOps.GetNRN(s.Nrn)
	if err != nil {
		return err
	}

	if err := d.Set("nrn", s.Nrn); err != nil {
		return err
	}

	if err := d.Set("scope_name", s.Name); err != nil {
		return err
	}

	if err := d.Set("scope_type", s.Type); err != nil {
		return err
	}

	if err := d.Set("null_application_id", s.ApplicationId); err != nil {
		return err
	}

	if err := d.Set("s3_assets_bucket", n.Namespaces.AWS.AWSS3AssestBucket); err != nil {
		return err
	}

	if err := d.Set("scope_workflow_role", n.Namespaces.AWS.AWSScopeWorkflowRole); err != nil {
		return err
	}

	if err := d.Set("log_group_name", n.Namespaces.AWS.AWSLogGroupName); err != nil {
		return err
	}

	if err := d.Set("lambda_function_name", n.Namespaces.AWS.AWSLambdaFunctionName); err != nil {
		return err
	}

	if err := d.Set("lambda_current_function_version", n.Namespaces.AWS.AWSLambdaCurrentFunctionVersion); err != nil {
		return err
	}

	if err := d.Set("lambda_function_role", n.Namespaces.AWS.AWSLambdaFunctionRole); err != nil {
		return err
	}

	if err := d.Set("lambda_function_main_alias", n.Namespaces.AWS.AWSLambdaFunctionMainAlias); err != nil {
		return err
	}

	if err := d.Set("log_reader_role", n.Namespaces.AWS.AWSLogReaderLog); err != nil {
		return err
	}

	if err := d.Set("lambda_function_warm_alias", n.Namespaces.AWS.AWSLambdaFunctionWarmAlias); err != nil {
		return err
	}

	if err := d.Set("capabilities_serverless_handler_name", s.Capabilities.ServerlessHandler["name"]); err != nil {
		return err
	}

	if err := d.Set("capabilities_serverless_timeout", s.Capabilities.ServerlessTimeout["timeout_in_seconds"]); err != nil {
		return err
	}

	if err := d.Set("capabilities_serverless_runtime_id", s.Capabilities.ServerlessRuntime["id"]); err != nil {
		return err
	}

	if err := d.Set("capabilities_serverless_ephemeral_storage", s.Capabilities.ServerlessEphemeralStorage["memory_in_mb"]); err != nil {
		return err
	}

	if err := d.Set("capabilities_serverless_memory", s.Capabilities.ServerlessMemory["memory_in_mb"]); err != nil {
		return err
	}

	if err := d.Set("runtime_configurations", s.RuntimeConfigurations); err != nil {
		return err
	}

	if err := d.Set("dimensions", s.Dimensions); err != nil {
		return err
	}

	return nil
}

func ScopeUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	scopeID := d.Id()

	log.Println("scopeID:", scopeID)

	ps := &Scope{}

	if d.HasChange("scope_name") {
		ps.Name = d.Get("scope_name").(string)
	}

	if d.HasChange("scope_type") {
		ps.Type = d.Get("scope_type").(string)
	}

	if d.HasChange("dimensions") {
		dimensionsMap := d.Get("dimensions").(map[string]interface{})

		// Convert the dimensions to a map[string]string
		dimensions := make(map[string]string)
		for key, value := range dimensionsMap {
			dimensions[key] = value.(string)
		}

		ps.Dimensions = dimensions
	}

	caps := &Capability{}

	if d.HasChange("capabilities_serverless_runtime_id") {
		caps.ServerlessRuntime = map[string]string{
			"provider": "aws_lambda",
			"id":       d.Get("capabilities_serverless_runtime_id").(string),
		}
	}

	if d.HasChange("capabilities_serverless_handler_name") {
		caps.ServerlessHandler = map[string]string{
			"name": d.Get("capabilities_serverless_handler_name").(string),
		}
	}

	if d.HasChange("capabilities_serverless_timeout") {
		caps.ServerlessTimeout = map[string]int{
			"timeout_in_seconds": d.Get("capabilities_serverless_timeout").(int),
		}
	}

	if d.HasChange("capabilities_serverless_ephemeral_storage") {
		caps.ServerlessEphemeralStorage = map[string]int{
			"memory_in_mb": d.Get("capabilities_serverless_ephemeral_storage").(int),
		}
	}

	if d.HasChange("capabilities_serverless_memory") {
		caps.ServerlessMemory = map[string]int{
			"memory_in_mb": d.Get("capabilities_serverless_memory").(int),
		}
	}

	if !reflect.DeepEqual(caps, Capability{}) {
		ps.Capabilities = caps
	}

	// Optional values can be updated as empty values
	if d.HasChange("s3_assets_bucket") || d.HasChange("scope_workflow_role") || d.HasChange("log_group_name") ||
		d.HasChange("lambda_function_name") || d.HasChange("lambda_current_function_version") || d.HasChange("lambda_function_role") ||
		d.HasChange("lambda_function_main_alias") || d.HasChange("log_reader_role") || d.HasChange("lambda_function_warm_alias") {

		nrnErr := patchNrnForScope(d.Get("nrn").(string), d, m)
		if nrnErr != nil {
			return nrnErr
		}

	}

	if !reflect.DeepEqual(*ps, Scope{}) {
		err := nullOps.PatchScope(scopeID, ps)
		if err != nil {
			return err
		}
	}

	return ScopeRead(d, m)
}

func ScopeDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	scopeID := d.Id()

	pScope := &Scope{
		Status: "deleting",
	}

	err := nullOps.PatchScope(scopeID, pScope)
	if err != nil {
		return err
	}

	pScope.Status = "deleted"

	err = nullOps.PatchScope(scopeID, pScope)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}

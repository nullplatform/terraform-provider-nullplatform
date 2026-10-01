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
		Description: "The scope resource allows you to configure a Nullplatform Scope",

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
				Type:        schema.TypeString,
				Computed:    true,
				Description: "A system-wide unique ID representing the resource.",
			},
			"scope_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The scope name.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Short description of the scope.",
			},
			"scope_type": {
				Type:        schema.TypeString,
				Default:     "serverless",
				Optional:    true,
				Description: "Possible values: [`web_pool`, `scheduled_tasks`, `serverless`]. Defaults to `serverless`.",
			},
			"scope_asset_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The asset name for the scope.",
			},
			"null_application_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "The ID of the application that owns this scope.",
			},
			"s3_assets_bucket": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The AWS S3 bucket name where the assets are stored (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"scope_workflow_role": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The ARN of the IAM Role to deploy new versions of the Scope (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"log_group_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The CloudWatch log group your Lambda function sends logs to (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"lambda_function_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The unique name of your Lambda function (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"lambda_current_function_version": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The version number of the Lambda function used as the baseline for Null Platform to create new function versions (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"lambda_function_role": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The ARN of the function's execution role (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"lambda_function_main_alias": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The Lambda function main ALIAS name (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"log_reader_role": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The ARN of the IAM Role to read CloudWatch logs (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"lambda_function_warm_alias": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The Lambda function ALIAS name used to warmup the function (NRN key). When omitted, the value already held by the scope's NRN is kept.",
				Deprecated:  "Configure NRN using the 'nullplatform_provider_config' resource instead.",
			},
			"capabilities_serverless_handler_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The function entrypoint in your code.",
			},
			"capabilities_serverless_timeout": {
				Type:        schema.TypeInt,
				Default:     10,
				Optional:    true,
				Description: "Amount of time your Lambda Function has to run in seconds. Defaults to `10`.",
			},
			"capabilities_serverless_runtime_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Identifier of the function's runtime. See [Runtimes](https://docs.aws.amazon.com/lambda/latest/api/API_CreateFunction.html#lambda-CreateFunction-request-Runtime) for valid values.",
			},
			"capabilities_serverless_runtime_platform": {
				Type:        schema.TypeString,
				Default:     "x86_64",
				Optional:    true,
				Description: "Instruction set architecture for your Lambda function. Valid values are `x86_64`, and `arm_64`.",
			},
			"capabilities_serverless_ephemeral_storage": {
				Type:        schema.TypeInt,
				Default:     512,
				Optional:    true,
				Description: "The amount of Ephemeral storage (`/tmp`) to allocate for the Lambda Function in MB. This parameter is used to expand the total amount of Ephemeral storage available, beyond the default amount of `512MB`.",
			},
			"capabilities_serverless_memory": {
				Type:        schema.TypeInt,
				Default:     128,
				Optional:    true,
				Description: "Amount of memory in MB your Lambda Function can use at runtime. Defaults to `128`. See [Limits](https://docs.aws.amazon.com/lambda/latest/dg/limits.html)",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A key-value map with the runtime configuration dimensions that apply to this scope.",
			},
			"runtime_configurations": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Description: "List of the runtime configurations that apply to this scope based on its dimensions and values.",
			},
		},
	}
}

func ScopeCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	applicationId := d.Get("null_application_id").(int)
	scopeName := d.Get("scope_name").(string)
	scopeType := d.Get("scope_type").(string)
	scopeAssetName := d.Get("scope_asset_name").(string)
	serverless_runtime := d.Get("capabilities_serverless_runtime_id").(string)
	serverless_platform := d.Get("capabilities_serverless_runtime_platform").(string)
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
		Description:     d.Get("description").(string),
		ApplicationId:   applicationId,
		Type:            scopeType,
		AssetName:       scopeAssetName,
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
				"platform": serverless_platform,
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

	// Tracked before the NRN patch, so a failed patch taints the scope instead of orphaning it.
	d.SetId(strconv.Itoa(s.Id))

	nrnErr := patchNrnForScope(s.Nrn, d, m)

	if nrnErr != nil {
		return nrnErr
	}

	return ScopeRead(d, m)
}

func patchNrnForScope(scopeNrn string, d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	s3AssetsBucket := configuredString(d, "s3_assets_bucket")
	scopeWorkflowRole := configuredString(d, "scope_workflow_role")
	logGroupName := configuredString(d, "log_group_name")
	lambdaFunctinoName := configuredString(d, "lambda_function_name")
	lambdaCurrentFunctionVersion := configuredString(d, "lambda_current_function_version")
	lambdaFunctionRole := configuredString(d, "lambda_function_role")
	lambdaFunctionMainAlias := configuredString(d, "lambda_function_main_alias")
	logReaderRole := configuredString(d, "log_reader_role")
	lambdaFunctionWarmAlias := configuredString(d, "lambda_function_warm_alias")

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

	if !reflect.DeepEqual(*nrnReq, PatchNRN{}) {
		return nullOps.PatchNRN(scopeNrn, nrnReq)
	}

	return nil
}

func ScopeRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	scopeID := d.Id()

	s, err := nullOps.GetScope(scopeID)

	if err != nil {
		if s != nil && (s.Status == "deleted" || s.Status == "deleting") {
			d.SetId("")
			return nil
		}
		return err
	}

	n, err := nullOps.GetNRN(s.Nrn)
	if err != nil {
		return err
	}

	// A scope whose NRN keys were never written has no aws namespace.
	aws := &NrnAwsNamespace{}
	if n.Namespaces != nil && n.Namespaces.AWS != nil {
		aws = n.Namespaces.AWS
	}

	if err := d.Set("nrn", s.Nrn); err != nil {
		return err
	}

	if err := d.Set("scope_name", s.Name); err != nil {
		return err
	}

	if err := d.Set("description", s.Description); err != nil {
		return err
	}

	if err := d.Set("scope_type", s.Type); err != nil {
		return err
	}

	if err := d.Set("scope_asset_name", s.AssetName); err != nil {
		return err
	}

	if err := d.Set("null_application_id", s.ApplicationId); err != nil {
		return err
	}

	if err := d.Set("s3_assets_bucket", aws.AWSS3AssestBucket); err != nil {
		return err
	}

	if err := d.Set("scope_workflow_role", aws.AWSScopeWorkflowRole); err != nil {
		return err
	}

	if err := d.Set("log_group_name", aws.AWSLogGroupName); err != nil {
		return err
	}

	if err := d.Set("lambda_function_name", aws.AWSLambdaFunctionName); err != nil {
		return err
	}

	if err := d.Set("lambda_current_function_version", aws.AWSLambdaCurrentFunctionVersion); err != nil {
		return err
	}

	if err := d.Set("lambda_function_role", aws.AWSLambdaFunctionRole); err != nil {
		return err
	}

	if err := d.Set("lambda_function_main_alias", aws.AWSLambdaFunctionMainAlias); err != nil {
		return err
	}

	if err := d.Set("log_reader_role", aws.AWSLogReaderLog); err != nil {
		return err
	}

	if err := d.Set("lambda_function_warm_alias", aws.AWSLambdaFunctionWarmAlias); err != nil {
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

	if err := d.Set("capabilities_serverless_runtime_platform", s.Capabilities.ServerlessRuntime["platform"]); err != nil {
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

	if d.HasChange("description") {
		ps.Description = d.Get("description").(string)
	}

	if d.HasChange("scope_type") {
		ps.Type = d.Get("scope_type").(string)
	}

	if d.HasChange("scope_asset_name") {
		ps.AssetName = d.Get("scope_asset_name").(string)
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

	if d.HasChange("capabilities_serverless_runtime_id") || d.HasChange("capabilities_serverless_runtime_platform") {
		caps.ServerlessRuntime = map[string]string{
			"provider": "aws_lambda",
			"id":       d.Get("capabilities_serverless_runtime_id").(string),
			"platform": d.Get("capabilities_serverless_runtime_platform").(string),
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

	if !reflect.DeepEqual(*caps, Capability{}) {
		ps.Capabilities = caps
	}

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
	scopeId := d.Id()

	err := nullOps.DeleteScope(scopeId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}

package main

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceProviderConfigAwsconfiguration() *schema.Resource {
	return &schema.Resource{
		Description: "AWS",

		Create: providerConfigAwsconfigurationCreate,
		Read:   providerConfigAwsconfigurationRead,
		Update: providerConfigAwsconfigurationUpdate,
		Delete: ProviderConfigDelete,

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A key-value map with the provider dimensions that apply to this scope.",
			},
			"iam": {
				Type:        schema.TypeList,
				Description: "AWS IAM roles and permissions configuration",
				MaxItems:    1,
				Required:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"scope_workflow_role": {
							Type:        schema.TypeString,
							Description: "IAM role ARN used to perform actions over client cloud resources",
						},
						"scope_workflow_intermediate_role": {
							Type:        schema.TypeString,
							Description: "IAM role ARN that is intermediately assumed during workflow execution",
						},
					},
				},
			},
			"account": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"region": {
							Type:        schema.TypeString,
							Description: "The primary AWS region where your resources are deployed (e.g., 'us-east-1')",
						},
						"id": {
							Type:        schema.TypeString,
							Description: "A 12-digit number uniquely identifying your AWS account (e.g., '123456789012')",
						},
					},
				},
			},
			"storage": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"s3_assets_bucket": {
							Type:        schema.TypeString,
							Description: "The name of the Amazon S3 bucket used for storing build assets, or other static files",
						},
						"s3_params_bucket": {
							Type:        schema.TypeString,
							Description: "The name of the Amazon S3 bucket used for storing application parameters",
						},
					},
				},
			},
			"networking": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hosted_zone_id": {
							Type:        schema.TypeString,
							Description: "The Route53 private hosted zone ID",
						},
						"application_domain": {
							Type:        schema.TypeBool,
							Description: "Use account name as part of applications domains",
						},
						"security_group_ids": {
							Type:        schema.TypeList,
							Description: "A list of AWS Security Group IDs used to control inbound and outbound traffic (e.g., ['sg-0a1b2c3d'])",
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"hosted_public_zone_id": {
							Type:        schema.TypeString,
							Description: "The Route53 public hosted zone ID",
						},
						"vpc_id": {
							Type:        schema.TypeString,
							Description: "The identifier of the AWS Virtual Private Cloud (VPC) where your resources are deployed (e.g., 'vpc-1a2b3c4d')",
						},
						"subnet_ids": {
							Type:        schema.TypeList,
							Description: "A list of AWS Subnet IDs associated with your VPC (e.g., ['subnet-1234abcd', 'subnet-5678efgh'])",
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"domain_name": {
							Type:        schema.TypeString,
							Description: "The domain name to be used for when creating DNS resources",
						},
					},
				},
			},
		}),

		CustomizeDiff: CustomizeNRNDiff,
	}
}

func providerConfigAwsconfigurationCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else {
		nrn, err = ConstructNRNFromComponents(d, nullOps)
		if err != nil {
			return fmt.Errorf("error constructing NRN: %v %s", err, nrn)
		}
	}

	dimensionsMap := d.Get("dimensions").(map[string]interface{})
	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

	// Build attributes from individual fields
	attributes := make(map[string]interface{})

	if v, ok := d.GetOk("account"); ok {
		attributes["account"] = v
	}

	if v, ok := d.GetOk("iam"); ok {
		attributes["iam"] = v
	}

	if v, ok := d.GetOk("networking"); ok {
		attributes["networking"] = v
	}

	if v, ok := d.GetOk("storage"); ok {
		attributes["storage"] = v
	}

	// Get specification ID for this provider type
	specificationId, err := nullOps.GetSpecificationIdFromSlug("aws-configuration", nrn)
	if err != nil {
		return fmt.Errorf("error fetching specification ID for aws-configuration: %v", err)
	}

	newProviderConfig := &ProviderConfig{
		Nrn:             nrn,
		Dimensions:      dimensions,
		SpecificationId: specificationId,
		Attributes:      attributes,
	}

	pc, err := nullOps.CreateProviderConfig(newProviderConfig)
	if err != nil {
		return err
	}

	d.SetId(pc.Id)
	return providerConfigAwsconfigurationRead(d, m)
}

func providerConfigAwsconfigurationRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	providerConfigId := d.Id()

	pc, err := nullOps.GetProviderConfig(providerConfigId)
	if err != nil {
		return err
	}

	if err := d.Set("nrn", pc.Nrn); err != nil {
		return err
	}

	if err := d.Set("dimensions", pc.Dimensions); err != nil {
		return err
	}

	// Verify this is the correct provider type
	specificationSlug, err := nullOps.GetSpecificationSlugFromId(pc.SpecificationId)
	if err != nil {
		return fmt.Errorf("error fetching specification slug for ID %s: %v", pc.SpecificationId, err)
	}
	if specificationSlug != "aws-configuration" {
		return fmt.Errorf("provider configuration type mismatch: expected aws-configuration, got %s", specificationSlug)
	}

	// Set individual fields from attributes
	for key, value := range pc.Attributes {
		if err := d.Set(key, value); err != nil {
			return fmt.Errorf("error setting %s: %v", key, err)
		}
	}

	return nil
}

func providerConfigAwsconfigurationUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	providerConfigId := d.Id()

	pc := &ProviderConfig{}

	// Check which fields have changed and update attributes accordingly
	attributes := make(map[string]interface{})

	if d.HasChange("account") {
		if v, ok := d.GetOk("account"); ok {
			attributes["account"] = v
		}
	}

	if d.HasChange("iam") {
		if v, ok := d.GetOk("iam"); ok {
			attributes["iam"] = v
		}
	}

	if d.HasChange("networking") {
		if v, ok := d.GetOk("networking"); ok {
			attributes["networking"] = v
		}
	}

	if d.HasChange("storage") {
		if v, ok := d.GetOk("storage"); ok {
			attributes["storage"] = v
		}
	}

	if len(attributes) > 0 {
		pc.Attributes = attributes
	}

	err := nullOps.PatchProviderConfig(providerConfigId, pc)
	if err != nil {
		return err
	}

	return providerConfigAwsconfigurationRead(d, m)
}

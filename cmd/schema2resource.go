package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// JSONSchemaProperty represents a property in the JSON schema
type JSONSchemaProperty struct {
	Type        string                        `json:"type"`
	Description string                        `json:"description"`
	Properties  map[string]JSONSchemaProperty `json:"properties"`
	Items       *JSONSchemaProperty           `json:"items"`
	Enum        []string                      `json:"enum"`
}

// JSONSchema represents the main JSON schema structure
type JSONSchema struct {
	Title        string                        `json:"title"`
	ProviderType string                        `json:"providerType"`
	Type         string                        `json:"type"`
	Properties   map[string]JSONSchemaProperty `json:"properties"`
}

// Convert JSON schema types to Terraform schema types
func getSchemaType(jsonType string) string {
	switch jsonType {
	case "string":
		return "schema.TypeString"
	case "number":
		return "schema.TypeFloat"
	case "integer":
		return "schema.TypeInt"
	case "boolean":
		return "schema.TypeBool"
	case "array":
		return "schema.TypeList"
	case "object":
		return "schema.TypeList" // For nested objects we'll use TypeList with MaxItems: 1
	default:
		return "schema.TypeString"
	}
}

// Generate schema fields recursively
func generateSchemaFields(props map[string]JSONSchemaProperty) string {
	var fields []string

	for name, prop := range props {
		field := fmt.Sprintf(`"%s": {
				Type:        %s,`, name, getSchemaType(prop.Type))

		if prop.Description != "" {
			field += fmt.Sprintf(`
				Description: %q,`, prop.Description)
		}

		// For objects, create a nested schema
		if prop.Type == "object" && len(prop.Properties) > 0 {
			field += `
				MaxItems:    1,
				Required:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						` + generateSchemaFields(prop.Properties) + `
					},
				},`
		} else if prop.Type == "array" && prop.Items != nil {
			// For arrays, handle the items schema
			field += fmt.Sprintf(`
				Elem: &schema.Schema{
					Type: %s,
				},`, getSchemaType(prop.Items.Type))
		}

		field += `
			}`
		fields = append(fields, field)
	}

	return strings.Join(fields, ",\n")
}

const resourceTemplate = `
package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceProviderConfig{{.TypeName}}() *schema.Resource {
	return &schema.Resource{
		Description: "{{.Description}}",

		Create: providerConfig{{.TypeName}}Create,
		Read:   providerConfig{{.TypeName}}Read,
		Update: providerConfig{{.TypeName}}Update,
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
			{{.SchemaFields}}
		}),

		CustomizeDiff: CustomizeNRNDiff,
	}
}

func providerConfig{{.TypeName}}Create(d *schema.ResourceData, m interface{}) error {
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
	{{range $name, $prop := .Schema.Properties}}
	if v, ok := d.GetOk("{{$name}}"); ok {
		attributes["{{$name}}"] = v
	}
	{{end}}

	// Get specification ID for this provider type
	specificationId, err := nullOps.GetSpecificationIdFromSlug("{{.ProviderType}}", nrn)
	if err != nil {
		return fmt.Errorf("error fetching specification ID for {{.ProviderType}}: %v", err)
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
	return providerConfig{{.TypeName}}Read(d, m)
}

func providerConfig{{.TypeName}}Read(d *schema.ResourceData, m interface{}) error {
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
	if specificationSlug != "{{.ProviderType}}" {
		return fmt.Errorf("provider configuration type mismatch: expected {{.ProviderType}}, got %s", specificationSlug)
	}

	// Set individual fields from attributes
	for key, value := range pc.Attributes {
		if err := d.Set(key, value); err != nil {
			return fmt.Errorf("error setting %s: %v", key, err)
		}
	}

	return nil
}

func providerConfig{{.TypeName}}Update(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	providerConfigId := d.Id()

	pc := &ProviderConfig{}

	// Check which fields have changed and update attributes accordingly
	attributes := make(map[string]interface{})
	{{range $name, $prop := .Schema.Properties}}
	if d.HasChange("{{$name}}") {
		if v, ok := d.GetOk("{{$name}}"); ok {
			attributes["{{$name}}"] = v
		}
	}
	{{end}}

	if len(attributes) > 0 {
		pc.Attributes = attributes
	}

	err := nullOps.PatchProviderConfig(providerConfigId, pc)
	if err != nil {
		return err
	}

	return providerConfig{{.TypeName}}Read(d, m)
}
`

func generateResource(schema JSONSchema) error {
	if schema.ProviderType == "" {
		return fmt.Errorf("schema is missing required 'providerType' field")
	}

	// Convert provider type to a valid Go identifier
	typeName := strings.NewReplacer(
		"-", "_",
		".", "_",
	).Replace(schema.ProviderType)
	typeName = strings.Title(strings.Replace(typeName, "_", "", -1))

	// Generate schema fields
	schemaFields := generateSchemaFields(schema.Properties)

	data := struct {
		TypeName     string
		Description  string
		ProviderType string
		SchemaFields string
		Schema       JSONSchema
	}{
		TypeName:     typeName,
		Description:  schema.Title,
		ProviderType: schema.ProviderType,
		SchemaFields: schemaFields,
		Schema:       schema,
	}

	// Parse and execute template
	tmpl, err := template.New("resource").Parse(resourceTemplate)
	if err != nil {
		return err
	}

	// Create output file
	filename := fmt.Sprintf("resource_provider_%s.go", strings.Replace(schema.ProviderType, "-", "_", -1))
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: schema2resource <schema-dir>")
		os.Exit(1)
	}

	schemaDir := os.Args[1]

	// Process all JSON files in the schema directory
	files, err := filepath.Glob(filepath.Join(schemaDir, "*.json"))
	if err != nil {
		fmt.Printf("Error finding schema files: %v\n", err)
		os.Exit(1)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading schema file %s: %v\n", file, err)
			continue
		}

		var schema JSONSchema
		if err := json.Unmarshal(data, &schema); err != nil {
			fmt.Printf("Error parsing schema %s: %v\n", file, err)
			continue
		}

		if err := generateResource(schema); err != nil {
			fmt.Printf("Error generating resource from %s: %v\n", file, err)
			continue
		}

		fmt.Printf("Successfully generated resource from %s\n", file)
	}
}

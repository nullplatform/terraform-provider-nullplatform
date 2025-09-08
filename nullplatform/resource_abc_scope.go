package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceABCScope() *schema.Resource {
	return &schema.Resource{
		Description: "The abc_scope resource allows you to create agent-backed-scopes by automatically fetching specifications from GitHub repositories and creating the necessary scope types, service specifications, and action specifications.",

		CreateContext: ABCScopeCreate,
		ReadContext:   ABCScopeRead,
		UpdateContext: ABCScopeUpdate,
		DeleteContext: ABCScopeDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the agent-backed scope",
			},
			"source": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "GitHub repository URL with specs (e.g., 'github.com/nullplatform/scopes/k8s' or 'github.com/nullplatform/scopes/k8s#feature-branch')",
			},
			"provider_config": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Provider configuration for the scope",
			},
			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of tags for the scope",
			},
			"created_scope_type_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ID of the created scope type",
			},
			"created_service_spec_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ID of the created service specification",
			},
			"created_action_spec_ids": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "IDs of the created action specifications",
			},
			"github_specs": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON representation of the fetched GitHub specs (for debugging)",
			},
		}),
	}
}

// GitHubSpec represents the structure of specs fetched from GitHub
type GitHubSpec struct {
	ScopeType           map[string]interface{} `json:"scope_type,omitempty"`
	ServiceSpecification map[string]interface{} `json:"service_spec,omitempty"`
	ActionSpecifications []map[string]interface{} `json:"action_specs,omitempty"`
}

// parseGitHubURL parses a GitHub URL and extracts owner, repo, path, and branch
func parseGitHubURL(githubURL string) (owner, repo, repoPath, branch string, err error) {
	// Handle the case where URL starts with "github.com" (no protocol)
	if strings.HasPrefix(githubURL, "github.com") {
		githubURL = "https://" + githubURL
	}

	// Parse for branch (after #)
	parts := strings.Split(githubURL, "#")
	if len(parts) > 1 {
		branch = parts[1]
		githubURL = parts[0]
	} else {
		branch = "main" // default branch
	}

	// Parse URL
	u, err := url.Parse(githubURL)
	if err != nil {
		return "", "", "", "", fmt.Errorf("invalid GitHub URL: %v", err)
	}

	if u.Host != "github.com" {
		return "", "", "", "", fmt.Errorf("URL must be from github.com")
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 2 {
		return "", "", "", "", fmt.Errorf("GitHub URL must contain owner and repository")
	}

	owner = pathParts[0]
	repo = pathParts[1]
	
	// Everything after owner/repo is the path within the repo
	if len(pathParts) > 2 {
		repoPath = strings.Join(pathParts[2:], "/")
	}

	return owner, repo, repoPath, branch, nil
}

// fetchGitHubSpecs fetches specs from GitHub repository
func fetchGitHubSpecs(githubURL string) (*GitHubSpec, error) {
	owner, repo, repoPath, branch, err := parseGitHubURL(githubURL)
	if err != nil {
		return nil, err
	}

	// Build GitHub raw content URLs for specs
	baseURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", owner, repo, branch)
	
	// Add the repo path and specs directory
	specsPath := path.Join(repoPath, "specs")
	
	spec := &GitHubSpec{}

	// Try to fetch scope type spec template (correct filename)
	scopeTypeURL := fmt.Sprintf("%s/%s/scope-type-definition.json.tpl", baseURL, specsPath)
	if scopeTypeData, err := fetchAndProcessTemplate(scopeTypeURL, githubURL); err == nil {
		spec.ScopeType = scopeTypeData
	}

	// Try to fetch service specification template
	serviceSpecURL := fmt.Sprintf("%s/%s/service-spec.json.tpl", baseURL, specsPath)
	if serviceSpecData, err := fetchAndProcessTemplate(serviceSpecURL, githubURL); err == nil {
		spec.ServiceSpecification = serviceSpecData
	}

	// Try to fetch ALL action specifications from actions/ directory
	actionsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/actions?ref=%s", owner, repo, specsPath, branch)
	if actionFiles, err := fetchGitHubDirectoryListing(actionsURL); err == nil {
		for _, actionFile := range actionFiles {
			if strings.HasSuffix(actionFile, ".json.tpl") {
				actionURL := fmt.Sprintf("%s/%s/actions/%s", baseURL, specsPath, actionFile)
				if actionData, err := fetchAndProcessTemplate(actionURL, githubURL); err == nil {
					spec.ActionSpecifications = append(spec.ActionSpecifications, actionData)
				}
			}
		}
	}

	return spec, nil
}

// fetchAndProcessTemplate fetches a template file and processes it
func fetchAndProcessTemplate(url, githubURL string) (map[string]interface{}, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s: status %d", url, resp.StatusCode)
	}

	// Read the template content
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read template from %s: %v", url, err)
	}
	templateContent := string(bodyBytes)

	// Process the template with basic variable substitution
	processed := processTemplate(templateContent, githubURL)

	// Parse the processed JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(processed), &data); err != nil {
		return nil, fmt.Errorf("failed to decode processed template from %s: %v", url, err)
	}

	// Remove problematic fields that the API doesn't accept
	delete(data, "status")
	delete(data, "id") // Remove any id field from templates

	return data, nil
}

// processTemplate performs basic template variable substitution
func processTemplate(template, githubURL string) string {
	// Extract variables from context (for now, use hardcoded values)
	// TODO: This should be made more sophisticated with proper template engine
	
	// Replace common template variables
	processed := strings.ReplaceAll(template, "{{ (default (env.Getenv \"USER\") (env.Getenv \"NAME\")) }}", "abc-scope")
	processed = strings.ReplaceAll(processed, "{{ env.Getenv \"NRN\" }}", "organization=1255165411:account=95118862:namespace=965608594:application=6171252")
	processed = strings.ReplaceAll(processed, "{{ env.Getenv \"SERVICE_SPECIFICATION_ID\" }}", "placeholder-service-spec-id")
	
	return processed
}

// fetchGitHubDirectoryListing fetches file names from GitHub API
func fetchGitHubDirectoryListing(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch directory listing from %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch directory listing from %s: status %d", url, resp.StatusCode)
	}

	var files []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode directory listing from %s: %v", url, err)
	}

	var fileNames []string
	for _, file := range files {
		if file.Type == "file" {
			fileNames = append(fileNames, file.Name)
		}
	}

	return fileNames, nil
}

// fetchJSONFromURL fetches JSON data from a URL (kept for backward compatibility)
func fetchJSONFromURL(url string) (map[string]interface{}, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s: status %d", url, resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode JSON from %s: %v", url, err)
	}

	return data, nil
}

func ABCScopeCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	log.Printf("[DEBUG] ABC_SCOPE: Starting creation for resource %s", d.Get("name").(string))

	// Fetch specifications from GitHub
	githubURL := d.Get("source").(string)
	log.Printf("[DEBUG] ABC_SCOPE: Fetching GitHub specs from %s", githubURL)
	
	specs, err := fetchGitHubSpecs(githubURL)
	if err != nil {
		log.Printf("[ERROR] ABC_SCOPE: Failed to fetch GitHub specs from %s: %v", githubURL, err)
		return diag.FromErr(fmt.Errorf("failed to fetch GitHub specs from %s: %v", githubURL, err))
	}

	log.Printf("[DEBUG] ABC_SCOPE: Successfully fetched specs. ScopeType: %v, ServiceSpec: %v, ActionSpecs: %d", 
		specs.ScopeType != nil, specs.ServiceSpecification != nil, len(specs.ActionSpecifications))
	
	// Log the raw specs
	specsDebug, _ := json.Marshal(specs)
	log.Printf("[DEBUG] ABC_SCOPE: Raw fetched specs: %s", string(specsDebug))

	// If no specs are found, create minimal default structures
	if specs.ScopeType == nil && specs.ServiceSpecification == nil && len(specs.ActionSpecifications) == 0 {
		log.Printf("[WARN] ABC_SCOPE: No specs found in GitHub repo, using defaults")
		// Create a basic scope type spec as fallback
		specs.ScopeType = map[string]interface{}{
			"type":          "custom",
			"description":   fmt.Sprintf("Agent-backed scope created from %s (no specs found, using defaults)", githubURL),
			"provider_type": "service",
		}
		// Also create a default service specification since scope type requires a provider_id
		specs.ServiceSpecification = map[string]interface{}{
			"type": "scope",
			"selectors": map[string]interface{}{
				"category":     "any",
				"provider":     "any",
				"sub_category": "any",
				"imported":     false,
			},
			"assignable_to": "any",
			"visible_to": []string{"organization=1255165411:account=95118862:namespace=965608594:application=6171252"},
		}
	}

	// Store fetched specs for debugging
	specsJSON, _ := json.Marshal(specs)
	d.Set("github_specs", string(specsJSON))

	var createdIDs struct {
		ScopeTypeID     string
		ServiceSpecID   string
		ActionSpecIDs   []string
	}

	// 1. Create Scope Type - use explicit spec or derive from service spec
	if specs.ScopeType != nil || specs.ServiceSpecification != nil {
		// If no explicit scope type spec, create one from the service spec
		if specs.ScopeType == nil {
			log.Printf("[DEBUG] ABC_SCOPE: No scope type spec found, deriving from service spec")
			specs.ScopeType = map[string]interface{}{
				"type":          "custom",
				"description":   fmt.Sprintf("Scope type derived from %s service specification", githubURL),
				"provider_type": "service",
			}
		}
		log.Printf("[DEBUG] ABC_SCOPE: Creating scope type with specs: %+v", specs.ScopeType)
		
		var nrn string
		if v, ok := d.GetOk("nrn"); ok {
			nrn = v.(string)
			log.Printf("[DEBUG] ABC_SCOPE: Using provided NRN: %s", nrn)
		} else {
			nrn, err = ConstructNRNFromComponents(d, nullOps)
			if err != nil {
				log.Printf("[ERROR] ABC_SCOPE: Failed to construct NRN: %v", err)
				return diag.FromErr(fmt.Errorf("error constructing NRN: %v", err))
			}
			log.Printf("[DEBUG] ABC_SCOPE: Constructed NRN: %s", nrn)
		}

		scopeType := &ScopeType{
			Nrn:         nrn,
			Name:        d.Get("name").(string),
			Type:        getStringFromSpec(specs.ScopeType, "type", "custom"),
			Description: getStringFromSpec(specs.ScopeType, "description", "Agent-backed scope created from "+githubURL),
			ProviderType: getStringFromSpec(specs.ScopeType, "provider_type", "service"),
			Status:      "active", // Always set to active like the existing resource
		}

		log.Printf("[DEBUG] ABC_SCOPE: Creating scope type with data: %+v", scopeType)

		// We'll need a service spec ID for the provider_id - create service spec first if it exists
		if specs.ServiceSpecification != nil {
			log.Printf("[DEBUG] ABC_SCOPE: Creating service specification with specs: %+v", specs.ServiceSpecification)
			
			serviceSpec := &ServiceSpecification{
				Name: d.Get("name").(string) + "-service",
				Type: getStringFromSpec(specs.ServiceSpecification, "type", "scope"),
			}

			// Handle visible_to
			if visibleToData, ok := specs.ServiceSpecification["visible_to"].([]interface{}); ok {
				visibleTo := make([]string, len(visibleToData))
				for i, v := range visibleToData {
					visibleTo[i] = v.(string)
				}
				serviceSpec.VisibleTo = visibleTo
			} else {
				// Default visible_to from NRN
				if nrn, ok := d.GetOk("nrn"); ok {
					serviceSpec.VisibleTo = []string{nrn.(string)}
				}
			}

			// Handle assignable_to
			if assignableTo, ok := specs.ServiceSpecification["assignable_to"].(string); ok {
				serviceSpec.AssignableTo = assignableTo
			}

			log.Printf("[DEBUG] ABC_SCOPE: Service spec data before selectors: %+v", serviceSpec)

			// Handle selectors
			if selectorsData, ok := specs.ServiceSpecification["selectors"].(map[string]interface{}); ok {
				selectors := &Selectors{}
				if category, ok := selectorsData["category"].(string); ok {
					selectors.Category = category
				}
				if provider, ok := selectorsData["provider"].(string); ok {
					selectors.Provider = provider
				}
				if subCategory, ok := selectorsData["sub_category"].(string); ok {
					selectors.SubCategory = subCategory
				}
				if imported, ok := selectorsData["imported"].(bool); ok {
					selectors.Imported = imported
				}
				serviceSpec.Selectors = selectors
			}

			// Handle attributes
			if attributesData, ok := specs.ServiceSpecification["attributes"]; ok {
				serviceSpec.Attributes = attributesData.(map[string]interface{})
			}

			log.Printf("[DEBUG] ABC_SCOPE: Final service spec data: %+v", serviceSpec)
			log.Printf("[DEBUG] ABC_SCOPE: Calling nullOps.CreateServiceSpecification...")

			createdServiceSpec, err := nullOps.CreateServiceSpecification(serviceSpec)
			if err != nil {
				log.Printf("[ERROR] ABC_SCOPE: Failed to create service specification: %v", err)
				return diag.FromErr(fmt.Errorf("failed to create service specification: %v", err))
			}
			
			log.Printf("[DEBUG] ABC_SCOPE: Successfully created service spec with ID: %s", createdServiceSpec.Id)
			createdIDs.ServiceSpecID = createdServiceSpec.Id
			scopeType.ProviderId = createdServiceSpec.Id
		}

		log.Printf("[DEBUG] ABC_SCOPE: Final scope type data before API call: %+v", scopeType)
		
		// Log the JSON that will be sent to the API
		scopeTypeJSON, _ := json.Marshal(scopeType)
		log.Printf("[DEBUG] ABC_SCOPE: Scope type JSON being sent to API: %s", string(scopeTypeJSON))
		
		log.Printf("[DEBUG] ABC_SCOPE: Calling nullOps.CreateScopeType...")

		createdScopeType, err := nullOps.CreateScopeType(scopeType)
		if err != nil {
			log.Printf("[ERROR] ABC_SCOPE: Failed to create scope type: %v", err)
			return diag.FromErr(fmt.Errorf("failed to create scope type: %v", err))
		}
		
		log.Printf("[DEBUG] ABC_SCOPE: Successfully created scope type with ID: %d", createdScopeType.Id)
		createdIDs.ScopeTypeID = fmt.Sprintf("%d", createdScopeType.Id)
	}

	// 2. Create Action Specifications
	if len(specs.ActionSpecifications) > 0 && createdIDs.ServiceSpecID != "" {
		for _, actionSpecData := range specs.ActionSpecifications {
			actionSpec := &ActionSpecification{
				Name:                   getStringFromSpec(actionSpecData, "name", "action"),
				Type:                   getStringFromSpec(actionSpecData, "type", "custom"),
				ServiceSpecificationId: createdIDs.ServiceSpecID,
				Retryable:              getBoolFromSpec(actionSpecData, "retryable", false),
			}

			// Handle parameters
			if paramsData, ok := actionSpecData["parameters"]; ok {
				actionSpec.Parameters = paramsData.(map[string]interface{})
			}

			// Handle results
			if resultsData, ok := actionSpecData["results"]; ok {
				actionSpec.Results = resultsData.(map[string]interface{})
			}

			// Handle icon
			if icon, ok := actionSpecData["icon"].(string); ok {
				actionSpec.Icon = icon
			}

			// Handle annotations
			if annotationsData, ok := actionSpecData["annotations"]; ok {
				actionSpec.Annotations = annotationsData.(map[string]interface{})
			}

			createdActionSpec, err := nullOps.CreateActionSpecification(actionSpec)
			if err != nil {
				return diag.FromErr(fmt.Errorf("failed to create action specification: %v", err))
			}
			createdIDs.ActionSpecIDs = append(createdIDs.ActionSpecIDs, createdActionSpec.Id)
		}
	}

	// Set computed values
	if err := d.Set("created_scope_type_id", createdIDs.ScopeTypeID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("created_service_spec_id", createdIDs.ServiceSpecID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("created_action_spec_ids", createdIDs.ActionSpecIDs); err != nil {
		return diag.FromErr(err)
	}

	// Use the scope type ID as the resource ID - must have a valid ID
	if createdIDs.ScopeTypeID == "" {
		return diag.FromErr(fmt.Errorf("failed to create scope type - no ID returned"))
	}
	d.SetId(createdIDs.ScopeTypeID)

	return ABCScopeRead(ctx, d, m)
}

func ABCScopeRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	scopeTypeId := d.Id()

	// Read the created scope type to verify it exists
	scopeType, err := nullOps.GetScopeType(scopeTypeId)
	if err != nil {
		// Resource was deleted outside of Terraform
		d.SetId("")
		return nil
	}

	// Re-set all the resource attributes to ensure consistency
	if err := d.Set("name", scopeType.Name); err != nil {
		return diag.FromErr(err)
	}

	// The computed values should already be set from Create, but we need to ensure they persist
	// during Read operations. Since these are stored in the state, they should already be available.
	// If they're not set, that means this is a fresh read after import or similar.
	
	// For imports or other cases where computed values might be missing,
	// we'll keep the existing values if they exist in state
	return nil
}

func ABCScopeUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	// For now, most changes require recreation due to the complex nature of the resource
	// In a full implementation, you'd handle updates to individual child resources
	return diag.FromErr(fmt.Errorf("abc_scope resources cannot be updated in place. Please recreate the resource."))
}

func ABCScopeDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	// Delete in reverse order: action specs, service spec, scope type
	
	// Delete action specifications
	if actionSpecIDs, ok := d.GetOk("created_action_spec_ids"); ok {
		for _, id := range actionSpecIDs.([]interface{}) {
			actionSpecID := id.(string)
			serviceSpecID := d.Get("created_service_spec_id").(string)
			err := nullOps.DeleteActionSpecification(actionSpecID, "service", serviceSpecID)
			if err != nil {
				return diag.FromErr(fmt.Errorf("failed to delete action specification %s: %v", actionSpecID, err))
			}
		}
	}

	// Delete service specification
	if serviceSpecID, ok := d.GetOk("created_service_spec_id"); ok && serviceSpecID.(string) != "" {
		err := nullOps.DeleteServiceSpecification(serviceSpecID.(string))
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to delete service specification %s: %v", serviceSpecID.(string), err))
		}
	}

	// Delete scope type
	scopeTypeId := d.Id()
	err := nullOps.DeleteScopeType(scopeTypeId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete scope type %s: %v", scopeTypeId, err))
	}

	d.SetId("")
	return nil
}

// Helper functions to safely extract values from spec maps
func getStringFromSpec(spec map[string]interface{}, key, defaultValue string) string {
	if val, ok := spec[key].(string); ok {
		return val
	}
	return defaultValue
}

func getBoolFromSpec(spec map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := spec[key].(bool); ok {
		return val
	}
	return defaultValue
}
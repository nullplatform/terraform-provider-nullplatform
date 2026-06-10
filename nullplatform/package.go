package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const PACKAGE_PATH = "/packages"

// PackageComponent is one BOM entry: it pins an exact revision (snapshot) of
// a resource owned by another service. parent_id links action/link spec
// components to their owning spec component within the same BOM.
type PackageComponent struct {
	Name               string  `json:"name"`
	ResourceType       string  `json:"resource_type"`
	ResourceID         string  `json:"resource_id"`
	ResourceRevisionID string  `json:"resource_revision_id"`
	ParentID           *string `json:"parent_id,omitempty"`
}

// PackageUpsert is the PUT /packages body: an idempotent slug-keyed publish.
// Missing (nrn, slug) creates the package + first revision; existing ones
// publish a new revision. `default` promotes the published revision to the
// package default in the same call.
type PackageUpsert struct {
	Nrn        string             `json:"nrn"`
	Slug       string             `json:"slug"`
	Name       string             `json:"name,omitempty"`
	Version    string             `json:"version,omitempty"`
	Components []PackageComponent `json:"components,omitempty"`
	VisibleTo  []string           `json:"visible_to,omitempty"`
	Default    bool               `json:"default,omitempty"`
}

// PackagePatch carries the mutable envelope fields for PATCH /packages/:id.
// DefaultRevisionID pins which revision services bind to by default; it must
// belong to the package (the API validates) and is mutually exclusive with
// `default: true` on a publish body.
type PackagePatch struct {
	Name              string   `json:"name,omitempty"`
	VisibleTo         []string `json:"visible_to,omitempty"`
	DefaultRevisionID string   `json:"default_revision_id,omitempty"`
}

// Package is the envelope returned by the package read endpoints, with the
// resolved BOM of the default (or latest) revision inlined as components.
type Package struct {
	ID                string             `json:"id,omitempty"`
	Nrn               string             `json:"nrn,omitempty"`
	Slug              string             `json:"slug,omitempty"`
	Name              string             `json:"name,omitempty"`
	VisibleTo         []string           `json:"visible_to,omitempty"`
	DefaultRevisionID string             `json:"default_revision_id,omitempty"`
	LatestRevisionID  string             `json:"latest_revision_id,omitempty"`
	DefaultVersion    string             `json:"default_version,omitempty"`
	LatestVersion     string             `json:"latest_version,omitempty"`
	Components        []PackageComponent `json:"components,omitempty"`
}

// PackageRevision is one published, immutable revision of a package.
type PackageRevision struct {
	ID        string `json:"id,omitempty"`
	PackageID string `json:"package_id,omitempty"`
	Version   string `json:"version,omitempty"`
}

type packageListResponse struct {
	Results []*Package `json:"results"`
}

type packageRevisionListResponse struct {
	Results []*PackageRevision `json:"results"`
}

func (c *NullClient) UpsertPackage(p *PackageUpsert) (*Package, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(p); err != nil {
		return nil, fmt.Errorf("error encoding package: %v", err)
	}

	res, err := c.MakeRequest("PUT", PACKAGE_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("error making PUT request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// 201 on create, 200 on publish over an existing package.
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("API error publishing package: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return nil, fmt.Errorf("error publishing package, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	pkg := &Package{}
	if err := json.Unmarshal(body, pkg); err != nil {
		return nil, fmt.Errorf("error decoding package: %v", err)
	}

	return pkg, nil
}

func (c *NullClient) GetPackage(packageID string) (*Package, error) {
	path := fmt.Sprintf("%s/%s", PACKAGE_PATH, packageID)

	body, err := c.getJSON(path, "package")
	if err != nil {
		return nil, err
	}

	pkg := &Package{}
	if err := json.Unmarshal(body, pkg); err != nil {
		return nil, fmt.Errorf("error decoding package: %v", err)
	}

	return pkg, nil
}

func (c *NullClient) PatchPackage(packageID string, p *PackagePatch) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(p); err != nil {
		return fmt.Errorf("error encoding package patch: %v", err)
	}

	path := fmt.Sprintf("%s/%s", PACKAGE_PATH, packageID)

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("error making PATCH request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
			return fmt.Errorf("API error patching package: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return fmt.Errorf("error patching package, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	return nil
}

func (c *NullClient) DeletePackage(packageID string) error {
	path := fmt.Sprintf("%s/%s", PACKAGE_PATH, packageID)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("error making DELETE request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
			return fmt.Errorf("API error deleting package: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return fmt.Errorf("error deleting package, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	return nil
}

// FindPackage resolves a package by its natural key (nrn, slug).
func (c *NullClient) FindPackage(nrn, slug string) (*Package, error) {
	params := map[string]string{"nrn": nrn, "slug": slug}
	path := fmt.Sprintf("%s%s", PACKAGE_PATH, c.PrepareQueryString(params))

	body, err := c.getJSON(path, "packages")
	if err != nil {
		return nil, err
	}

	response := &packageListResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		return nil, fmt.Errorf("error decoding package list: %v", err)
	}

	if len(response.Results) != 1 {
		return nil, fmt.Errorf("expected exactly one package for nrn=%s slug=%s, got %d", nrn, slug, len(response.Results))
	}

	return response.Results[0], nil
}

func (c *NullClient) ListPackageRevisions(packageID string) ([]*PackageRevision, error) {
	path := fmt.Sprintf("%s/%s/revisions", PACKAGE_PATH, packageID)

	body, err := c.getJSON(path, "package revisions")
	if err != nil {
		return nil, err
	}

	response := &packageRevisionListResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		return nil, fmt.Errorf("error decoding package revision list: %v", err)
	}

	return response.Results, nil
}

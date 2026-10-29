package nullplatform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const PACKAGE_INSTALLATION_PATH = "/package_installations"

// errPackageInstallationNotFound is a GET answered 404: the installation was
// deleted (or never existed). Read drops it from state so the next plan offers
// to install again.
var errPackageInstallationNotFound = errors.New("package installation not found")

// PackageInstallationCreate is the POST /package_installations body. An
// installation is the organization's row for a package at one of its NRNs:
// where the package's specification is usable (visible_to, inside the
// installing organization) and which revision new services there are born
// with (default_revision_id). Omitted, default_revision_id resolves to the
// package's default at install time and is stored; it never follows the
// package's default afterwards.
type PackageInstallationCreate struct {
	Nrn               string   `json:"nrn"`
	PackageID         string   `json:"package_id"`
	DefaultRevisionID string   `json:"default_revision_id,omitempty"`
	VisibleTo         []string `json:"visible_to,omitempty"`
}

// PackageInstallationPatch moves an installation's default revision or its
// visibility. The package and the NRN are fixed: changing either is another
// installation.
type PackageInstallationPatch struct {
	DefaultRevisionID string   `json:"default_revision_id,omitempty"`
	VisibleTo         []string `json:"visible_to,omitempty"`
}

// MissingRequirement is an optional provider the package declares that the
// organization has not configured at the installation NRN or above it.
// Required ones refuse the install instead.
type MissingRequirement struct {
	Category      string `json:"category,omitempty"`
	Specification string `json:"specification,omitempty"`
}

type PackageInstallation struct {
	ID                          string               `json:"id,omitempty"`
	Nrn                         string               `json:"nrn,omitempty"`
	PackageID                   string               `json:"package_id,omitempty"`
	DefaultRevisionID           string               `json:"default_revision_id,omitempty"`
	DefaultVersion              string               `json:"default_version,omitempty"`
	VisibleTo                   []string             `json:"visible_to,omitempty"`
	CreatedBy                   string               `json:"created_by,omitempty"`
	MissingOptionalRequirements []MissingRequirement `json:"missing_optional_requirements,omitempty"`
}

func (c *NullClient) CreatePackageInstallation(body *PackageInstallationCreate) (*PackageInstallation, error) {
	return c.writePackageInstallation("POST", PACKAGE_INSTALLATION_PATH, body, "creating")
}

func (c *NullClient) GetPackageInstallation(id string) (*PackageInstallation, error) {
	res, err := c.MakeRequest("GET", fmt.Sprintf("%s/%s", PACKAGE_INSTALLATION_PATH, id), nil)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", errPackageInstallationNotFound, id)
	}
	return decodePackageInstallation(res, "getting")
}

func (c *NullClient) PatchPackageInstallation(id string, body *PackageInstallationPatch) (*PackageInstallation, error) {
	return c.writePackageInstallation("PATCH", fmt.Sprintf("%s/%s", PACKAGE_INSTALLATION_PATH, id), body, "updating")
}

// DeletePackageInstallation uninstalls. The API refuses (400) while a live
// service born from the installation sits under its visible_to; that message
// is the operator's answer and is returned verbatim. A 404 is already gone.
func (c *NullClient) DeletePackageInstallation(id string) error {
	res, err := c.MakeRequest("DELETE", fmt.Sprintf("%s/%s", PACKAGE_INSTALLATION_PATH, id), nil)
	if err != nil {
		return fmt.Errorf("error making DELETE request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusNoContent || res.StatusCode == http.StatusNotFound {
		return nil
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}
	return packageInstallationError(res.StatusCode, body, "deleting")
}

func (c *NullClient) writePackageInstallation(method, path string, payload any, verb string) (*PackageInstallation, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return nil, fmt.Errorf("error encoding package installation: %v", err)
	}
	res, err := c.MakeRequest(method, path, &buf)
	if err != nil {
		return nil, fmt.Errorf("error making %s request: %v", method, err)
	}
	defer res.Body.Close()
	return decodePackageInstallation(res, verb)
}

func decodePackageInstallation(res *http.Response, verb string) (*PackageInstallation, error) {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return nil, packageInstallationError(res.StatusCode, body, verb)
	}
	installation := &PackageInstallation{}
	if err := json.Unmarshal(body, installation); err != nil {
		return nil, fmt.Errorf("error decoding package installation: %v", err)
	}
	return installation, nil
}

// packageInstallationError carries the API's message: a refused install names
// what is wrong (the missing provider and the NRN, a deprecated revision, a
// package of another organization), and the operator needs it verbatim.
func packageInstallationError(status int, body []byte, verb string) error {
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
		return fmt.Errorf("API error %s package installation: %s", verb, errResp.Message)
	}
	return fmt.Errorf("error %s package installation, got status code: %d, body: %s", verb, status, string(body))
}

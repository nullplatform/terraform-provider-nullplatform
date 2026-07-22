package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	ARTIFACT_PATH          = "/artifacts"
	ARTIFACT_REVISION_PATH = "/artifact_revision"
)

// PlatformArtifactRegistration is the POST /artifacts request body. Register
// is an idempotent upsert: the per-type stable subset of meta identifies the
// artifact row, each unique meta blob mints (or reuses) one revision.
type PlatformArtifactRegistration struct {
	Nrn       string                 `json:"nrn"`
	Type      string                 `json:"type"`
	Meta      map[string]interface{} `json:"meta"`
	VisibleTo []string               `json:"visible_to,omitempty"`
}

// PlatformArtifactRevision is the (artifact, revision) pair returned by
// register and by the revision read endpoints. The ids are what package BOM
// components pin as resource_id / resource_revision_id.
type PlatformArtifactRevision struct {
	ResourceID         string                 `json:"resource_id,omitempty"`
	ResourceRevisionID string                 `json:"resource_revision_id,omitempty"`
	Type               string                 `json:"type,omitempty"`
	Meta               map[string]interface{} `json:"meta,omitempty"`
	Nrn                string                 `json:"nrn,omitempty"`
	VisibleTo          []string               `json:"visible_to,omitempty"`
	CreatedAt          string                 `json:"created_at,omitempty"`
}

// PlatformArtifact is the artifact envelope returned by GET /artifacts/:id.
type PlatformArtifact struct {
	ResourceID       string                 `json:"resource_id,omitempty"`
	Type             string                 `json:"type,omitempty"`
	Nrn              string                 `json:"nrn,omitempty"`
	IdentityMeta     map[string]interface{} `json:"identity_meta,omitempty"`
	VisibleTo        []string               `json:"visible_to,omitempty"`
	RevisionCount    int                    `json:"revision_count,omitempty"`
	LatestRevisionID string                 `json:"latest_revision_id,omitempty"`
	CreatedAt        string                 `json:"created_at,omitempty"`
}

type platformArtifactListResponse struct {
	Results []*PlatformArtifact `json:"results"`
}

type platformArtifactRevisionListResponse struct {
	Results []*PlatformArtifactRevision `json:"results"`
}

func (c *NullClient) RegisterPlatformArtifact(r *PlatformArtifactRegistration) (*PlatformArtifactRevision, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, fmt.Errorf("error encoding artifact registration: %v", err)
	}

	res, err := c.MakeRequest("POST", ARTIFACT_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("error making POST request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// 201 on first registration, 200 when the (artifact, revision) pair is reused.
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("API error registering artifact: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return nil, fmt.Errorf("error registering artifact, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	revision := &PlatformArtifactRevision{}
	if err := json.Unmarshal(body, revision); err != nil {
		return nil, fmt.Errorf("error decoding artifact registration response: %v", err)
	}

	return revision, nil
}

func (c *NullClient) GetPlatformArtifact(artifactID string) (*PlatformArtifact, error) {
	path := fmt.Sprintf("%s/%s", ARTIFACT_PATH, artifactID)

	body, err := c.getJSON(path, "artifact")
	if err != nil {
		return nil, err
	}

	artifact := &PlatformArtifact{}
	if err := json.Unmarshal(body, artifact); err != nil {
		return nil, fmt.Errorf("error decoding artifact: %v", err)
	}

	return artifact, nil
}

// GetPlatformArtifactRevision resolves a revision directly by id via the
// standalone GET /artifact_revision/:id endpoint — no parent artifact id
// needed.
func (c *NullClient) GetPlatformArtifactRevision(revisionID string) (*PlatformArtifactRevision, error) {
	path := fmt.Sprintf("%s/%s", ARTIFACT_REVISION_PATH, revisionID)

	body, err := c.getJSON(path, "artifact revision")
	if err != nil {
		return nil, err
	}

	revision := &PlatformArtifactRevision{}
	if err := json.Unmarshal(body, revision); err != nil {
		return nil, fmt.Errorf("error decoding artifact revision: %v", err)
	}

	return revision, nil
}

func (c *NullClient) ListPlatformArtifacts(nrn, artifactType string) ([]*PlatformArtifact, error) {
	params := map[string]string{}
	if nrn != "" {
		params["nrn"] = nrn
	}
	if artifactType != "" {
		params["type"] = artifactType
	}

	path := fmt.Sprintf("%s%s", ARTIFACT_PATH, c.PrepareQueryString(params))

	body, err := c.getJSON(path, "artifacts")
	if err != nil {
		return nil, err
	}

	response := &platformArtifactListResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		return nil, fmt.Errorf("error decoding artifact list: %v", err)
	}

	return response.Results, nil
}

func (c *NullClient) ListPlatformArtifactRevisions(artifactID string) ([]*PlatformArtifactRevision, error) {
	path := fmt.Sprintf("%s/%s/revisions", ARTIFACT_PATH, artifactID)

	body, err := c.getJSON(path, "artifact revisions")
	if err != nil {
		return nil, err
	}

	response := &platformArtifactRevisionListResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		return nil, fmt.Errorf("error decoding artifact revision list: %v", err)
	}

	return response.Results, nil
}

// getJSON performs a GET and returns the raw body on 200, mapping API error
// envelopes into readable errors otherwise.
func (c *NullClient) getJSON(path, entity string) ([]byte, error) {
	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("API error getting %s: %s (Code: %s)", entity, errResp.Message, errResp.Code)
		}
		return nil, fmt.Errorf("error getting %s, got status code: %d, body: %s", entity, res.StatusCode, string(body))
	}

	return body, nil
}

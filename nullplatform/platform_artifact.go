package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

// ListPlatformArtifacts lists the artifacts VISIBLE at nrn — not merely owned
// by it. The visible_to filter matches artifacts owned at the nrn (visible_to
// defaults to the owner), shared by ancestors or trailing-wildcard scopes, and
// globals published with "organization=*" — which an owner-nrn filter can
// never return, since globals are owned by another organization entirely.
func (c *NullClient) ListPlatformArtifacts(nrn, artifactType string) ([]*PlatformArtifact, error) {
	params := map[string]string{}
	if nrn != "" {
		params["visible_to"] = nrn
	}
	if artifactType != "" {
		params["type"] = artifactType
	}

	return listAllPages[PlatformArtifact](c, ARTIFACT_PATH, params, "artifacts")
}

func (c *NullClient) ListPlatformArtifactRevisions(artifactID string) ([]*PlatformArtifactRevision, error) {
	path := fmt.Sprintf("%s/%s/revisions", ARTIFACT_PATH, artifactID)
	return listAllPages[PlatformArtifactRevision](c, path, map[string]string{}, "artifact revisions")
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

// listPageSize is how many items a listing asks for per page.
const listPageSize = 100

// listAllPages walks a paged listing (`limit`/`offset`, `{ results }`) until a
// page comes back short, and returns every item. Lookups that search a listing
// must see all of it: the API answers 30 items by default, and trusting the
// first page made a lookup fail as soon as a listing grew past it.
func listAllPages[T any](c *NullClient, path string, params map[string]string, entity string) ([]*T, error) {
	var all []*T
	for offset := 0; ; offset += listPageSize {
		query := map[string]string{"limit": strconv.Itoa(listPageSize), "offset": strconv.Itoa(offset)}
		for k, v := range params {
			query[k] = v
		}
		body, err := c.getJSON(path+c.PrepareQueryString(query), entity)
		if err != nil {
			return nil, err
		}
		page := struct {
			Results []*T `json:"results"`
		}{}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("error decoding %s list: %v", entity, err)
		}
		all = append(all, page.Results...)
		if len(page.Results) < listPageSize {
			return all, nil
		}
	}
}

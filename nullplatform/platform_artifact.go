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
	Paging  *Paging             `json:"paging,omitempty"`
}

type platformArtifactRevisionListResponse struct {
	Results []*PlatformArtifactRevision `json:"results"`
	Paging  *Paging                     `json:"paging,omitempty"`
}

// platformArtifactPageSize is the page GET /artifacts and
// GET /artifacts/:id/revisions are read in. Both page by offset — 30 per page
// unless a limit is sent — and answer no total, so a listing is only whole
// when every page is read until a short one. Reading the first page alone
// truncated the visible set once the globals crossed 30 (2026-10-06): the
// older production artifacts fell to page two and every lookup of them
// failed as "not visible".
var platformArtifactPageSize = 200

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
// Every page of the listing is read.
func (c *NullClient) ListPlatformArtifacts(nrn, artifactType string) ([]*PlatformArtifact, error) {
	params := map[string]string{}
	if nrn != "" {
		params["visible_to"] = nrn
	}
	if artifactType != "" {
		params["type"] = artifactType
	}
	return listPlatformArtifactPages(c, ARTIFACT_PATH, params, "artifacts",
		func(artifact *PlatformArtifact) string { return artifact.ResourceID })
}

// ListPlatformArtifactRevisions lists every revision of an artifact, every
// page of them.
func (c *NullClient) ListPlatformArtifactRevisions(artifactID string) ([]*PlatformArtifactRevision, error) {
	path := fmt.Sprintf("%s/%s/revisions", ARTIFACT_PATH, artifactID)
	return listPlatformArtifactPages(c, path, map[string]string{}, "artifact revisions",
		func(revision *PlatformArtifactRevision) string { return revision.ResourceRevisionID })
}

// listPlatformArtifactPages reads a paged artifact listing whole: page after
// page from offset 0, platformArtifactPageSize at a time, until a page
// shorter than the limit the API echoes back (the API answers no total). The
// pages are keyed by offset, so an item registered between two reads shifts
// the rest and may come twice: idOf keeps it once.
func listPlatformArtifactPages[T any](c *NullClient, path string, params map[string]string, entity string, idOf func(T) string) ([]T, error) {
	var items []T
	seen := map[string]bool{}
	for offset := 0; ; {
		params["limit"] = strconv.Itoa(platformArtifactPageSize)
		params["offset"] = strconv.Itoa(offset)

		body, err := c.getJSON(path+c.PrepareQueryString(params), entity)
		if err != nil {
			return nil, err
		}

		var page struct {
			Results []T     `json:"results"`
			Paging  *Paging `json:"paging"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("error decoding %s list: %v", entity, err)
		}

		for _, item := range page.Results {
			if id := idOf(item); !seen[id] {
				seen[id] = true
				items = append(items, item)
			}
		}

		limit := platformArtifactPageSize
		if page.Paging != nil && page.Paging.Limit > 0 {
			limit = page.Paging.Limit
		}
		if len(page.Results) < limit {
			return items, nil
		}
		offset += len(page.Results)
	}
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

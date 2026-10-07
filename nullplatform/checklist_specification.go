package nullplatform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const CHECKLIST_SPECIFICATION_PATH = "/approval/checklist/specification"

// checklistSpecificationPageSize is how many lineages one list request asks
// for: the API's maximum.
var checklistSpecificationPageSize = 200

// errChecklistSpecificationNotFound is a request answered 404: no such version.
var errChecklistSpecificationNotFound = errors.New("checklist specification not found")

// ChecklistSpecification is one version of a lineage, (nrn, name). A PATCH
// inserts the next version under a new id; the old one stays readable.
// Definition is kept as the API answers it. Description is always sent: null
// clears it on a PATCH, where an absent field would be inherited.
type ChecklistSpecification struct {
	Id                     string                   `json:"id,omitempty"`
	Nrn                    string                   `json:"nrn,omitempty"`
	Name                   string                   `json:"name,omitempty"`
	Description            *string                  `json:"description"`
	Definition             json.RawMessage          `json:"definition,omitempty"`
	Status                 string                   `json:"status,omitempty"`
	Version                int                      `json:"version,omitempty"`
	Versions               []ChecklistSpecification `json:"versions,omitempty"`
	AssociatedActions      []ApprovalAction         `json:"associated_actions,omitempty"`
	AssociatedActionsCount int                      `json:"associated_actions_count,omitempty"`
}

// ChecklistSpecificationError is a request the API refused, with its problem
// document's message and detail (the detail names what failed: the
// validation errors of a 422, the version in use of a 409).
type ChecklistSpecificationError struct {
	Op         string
	StatusCode int
	Message    string
	Detail     string
}

func (e *ChecklistSpecificationError) Error() string {
	text := e.Message
	switch {
	case e.Detail == "":
	case strings.HasPrefix(e.Detail, e.Message):
		text = e.Detail
	default:
		text += ": " + e.Detail
	}
	// Redacted here, the one place every refusal is printed: a gateway may echo
	// the request, as a page or as a message.
	return redactMessage(fmt.Sprintf("error %s, got %d: %s", e.Op, e.StatusCode, text))
}

// checklistSpecificationNotFound is the API's 404 for a version it does not
// have.
const checklistSpecificationNotFound = "Checklist specification not found"

// Is reads as not found only the API's own 404: any other 404 (a gateway's
// page) is an error, never a specification gone.
func (e *ChecklistSpecificationError) Is(target error) bool {
	return target == errChecklistSpecificationNotFound && e.StatusCode == http.StatusNotFound && e.Message == checklistSpecificationNotFound
}

// checklistSpecificationRequest sends one request and decodes the answer into
// out (nil: no body), or returns the refusal. A body that is not a problem
// document (a gateway's HTML) becomes the message.
func (c *NullClient) checklistSpecificationRequest(op, method, path string, in any, want int, out any) error {
	var body *bytes.Buffer
	if in != nil {
		body = &bytes.Buffer{}
		if err := json.NewEncoder(body).Encode(in); err != nil {
			return err
		}
	}
	res, err := c.MakeRequest(method, path, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		var doc struct {
			Message string  `json:"message"`
			Detail  *string `json:"detail"`
		}
		refused := &ChecklistSpecificationError{Op: op, StatusCode: res.StatusCode, Message: strings.TrimSpace(string(raw))}
		if json.Unmarshal(raw, &doc) == nil && doc.Message != "" {
			refused.Message = doc.Message
			if doc.Detail != nil {
				refused.Detail = *doc.Detail
			}
		}
		return refused
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *NullClient) CreateChecklistSpecification(spec *ChecklistSpecification) (*ChecklistSpecification, error) {
	created := &ChecklistSpecification{}
	err := c.checklistSpecificationRequest("creating checklist specification "+spec.Nrn+"/"+spec.Name,
		http.MethodPost, CHECKLIST_SPECIFICATION_PATH, spec, http.StatusCreated, created)
	return created, err
}

// GetChecklistSpecification reads a version, deleted ones included, with its
// lineage (versions) and the actions linked to it.
func (c *NullClient) GetChecklistSpecification(id string) (*ChecklistSpecification, error) {
	spec := &ChecklistSpecification{}
	err := c.checklistSpecificationRequest("reading checklist specification "+id,
		http.MethodGet, CHECKLIST_SPECIFICATION_PATH+"/"+url.PathEscape(id)+"?include_versions=true", nil, http.StatusOK, spec)
	return spec, err
}

// PatchChecklistSpecification creates the next version of id's lineage and
// returns it.
func (c *NullClient) PatchChecklistSpecification(id string, spec *ChecklistSpecification) (*ChecklistSpecification, error) {
	next := &ChecklistSpecification{}
	err := c.checklistSpecificationRequest("updating checklist specification "+id,
		http.MethodPatch, CHECKLIST_SPECIFICATION_PATH+"/"+url.PathEscape(id), spec, http.StatusOK, next)
	return next, err
}

// DeleteChecklistSpecification soft-deletes one version.
func (c *NullClient) DeleteChecklistSpecification(id string) error {
	return c.checklistSpecificationRequest("deleting checklist specification "+id,
		http.MethodDelete, CHECKLIST_SPECIFICATION_PATH+"/"+url.PathEscape(id), nil, http.StatusNoContent, nil)
}

// ListChecklistSpecifications is the newest live version of every lineage at
// exactly nrn (no-merge: not its ancestors'), every page of it. The API pages
// by offset, so a lineage created between two pages may come twice: it is
// kept once.
func (c *NullClient) ListChecklistSpecifications(nrn string) ([]ChecklistSpecification, error) {
	var specs []ChecklistSpecification
	seen := map[string]bool{}
	for offset := 0; ; {
		query := url.Values{"nrn": {nrn}, "no-merge": {"true"}, "latest_only": {"true"},
			"limit": {strconv.Itoa(checklistSpecificationPageSize)}, "offset": {strconv.Itoa(offset)}}
		var page struct {
			Results []ChecklistSpecification `json:"results"`
			Total   int                      `json:"total"`
		}
		if err := c.checklistSpecificationRequest("listing checklist specifications of "+nrn,
			http.MethodGet, CHECKLIST_SPECIFICATION_PATH+"?"+query.Encode(), nil, http.StatusOK, &page); err != nil {
			return nil, err
		}
		for _, spec := range page.Results {
			if !seen[spec.Id] {
				seen[spec.Id] = true
				specs = append(specs, spec)
			}
		}
		offset += len(page.Results)
		if len(page.Results) == 0 || offset >= page.Total {
			return specs, nil
		}
	}
}

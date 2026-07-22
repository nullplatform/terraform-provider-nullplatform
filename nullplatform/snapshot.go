package nullplatform

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// specSnapshot is one append-only snapshot of a specification. The highest
// sequence_number is the newest; its id is what a package BOM component pins
// as resource_revision_id.
type specSnapshot struct {
	ID             string `json:"id"`
	SequenceNumber int    `json:"sequence_number"`
}

type specSnapshotList struct {
	Results []specSnapshot `json:"results"`
}

// GetLatestSnapshotID returns the newest snapshot id for a spec-like resource.
// kind is the URL segment of the owning service: "service_specification",
// "action_specification" or "link_specification". Returns "" (no error) when
// the resource has no snapshots yet, so callers can leave the computed
// attribute empty rather than failing the read.
func (c *NullClient) GetLatestSnapshotID(kind, id string) (string, error) {
	path := fmt.Sprintf("/%s/%s/snapshots", kind, id)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("error listing %s snapshots: %v", kind, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("error reading snapshot list response: %v", err)
	}

	if res.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("error listing %s snapshots, status code: %d, body: %s", kind, res.StatusCode, string(body))
	}

	list := &specSnapshotList{}
	if err := json.Unmarshal(body, list); err != nil {
		return "", fmt.Errorf("error decoding snapshot list: %v", err)
	}

	latestID := ""
	highest := -1
	for _, snapshot := range list.Results {
		if snapshot.SequenceNumber > highest {
			highest = snapshot.SequenceNumber
			latestID = snapshot.ID
		}
	}
	return latestID, nil
}

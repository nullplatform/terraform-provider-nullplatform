package nullplatform

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// listPageSize is the page a listing is read in. The listings page by
// offset — 30 per page unless a limit is sent — and answer no total, so a
// listing is only whole when every page is read until a short one. Reading
// the first page alone truncated the visible artifacts once the globals
// crossed 30 (2026-10-06): the older production artifacts fell to page two
// and every lookup of them failed as "not visible".
var listPageSize = 200

// listAllPages reads a paged listing whole: page after page from offset 0,
// listPageSize at a time, until a page shorter than the limit the API echoes
// back (the API answers no total). The pages are keyed by offset, so an item
// registered between two reads shifts the rest and may come twice: idOf keeps
// it once.
func listAllPages[T any](c *NullClient, path string, params map[string]string, entity string, idOf func(T) string) ([]T, error) {
	var items []T
	seen := map[string]bool{}
	for offset := 0; ; {
		params["limit"] = strconv.Itoa(listPageSize)
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

		limit := listPageSize
		if page.Paging != nil && page.Paging.Limit > 0 {
			limit = page.Paging.Limit
		}
		if len(page.Results) < limit {
			return items, nil
		}
		offset += len(page.Results)
	}
}

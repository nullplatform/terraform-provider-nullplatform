package nullplatform

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// jsonTokens is a JSON document as the sequence of its tokens: delimiters,
// keys and values in the order they are written. Whitespace, the way a string
// is escaped and the way a number is spelled are not part of it; the order of
// an object's keys is.
func jsonTokens(document string) ([]json.Token, bool) {
	// The token reader stops quietly where a document is cut short.
	if !json.Valid([]byte(document)) {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(document)))
	var tokens []json.Token
	for {
		token, err := decoder.Token()
		if err != nil {
			return tokens, true
		}
		tokens = append(tokens, token)
	}
}

// sameJSONInOrder reports whether two documents hold the same JSON with the
// keys of every object in the same order. A document that is not JSON equals
// nothing.
func sameJSONInOrder(a, b string) bool {
	tokensA, okA := jsonTokens(a)
	tokensB, okB := jsonTokens(b)
	return okA && okB && reflect.DeepEqual(tokensA, tokensB)
}

// suppressSameJSONInOrder is suppressEquivalentJSON for a document whose key
// order carries meaning: a change of whitespace is no diff, a change of order
// is one.
func suppressSameJSONInOrder(_, old, new string, _ *schema.ResourceData) bool {
	return sameJSONInOrder(old, new)
}

// compactJSON removes the whitespace between the tokens of a document and
// leaves everything else, the order of its keys included, as written.
func compactJSON(document []byte) (string, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, document); err != nil {
		return "", err
	}
	return compact.String(), nil
}

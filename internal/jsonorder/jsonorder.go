// Package jsonorder writes a JSON document with the keys of its objects in
// the order a JavaScript object keeps them: the platform's APIs parse what
// they are sent into one, and answer it back in that order.
package jsonorder

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
)

// JavaScript is the document, compact, with the keys of every object in
// JavaScript's order: the keys that read as array indexes first, in ascending
// order, then the others in the order they are written. A key written twice
// keeps its first place and its last value, as a JavaScript object does.
func JavaScript(document []byte) ([]byte, error) {
	var value orderedValue
	if err := json.Unmarshal(document, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

// orderedValue is a JSON value that keeps the keys of its objects in
// JavaScript's order.
type orderedValue struct {
	scalar json.RawMessage
	keys   []string
	values map[string]*orderedValue
	items  []*orderedValue
	kind   byte
}

func (v *orderedValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil
	}
	v.kind = trimmed[0]
	switch v.kind {
	case '{':
		v.values = map[string]*orderedValue{}
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		if _, err := decoder.Token(); err != nil {
			return err
		}
		var indexes, others []string
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			child := &orderedValue{}
			if err := decoder.Decode(child); err != nil {
				return err
			}
			name := key.(string)
			if _, seen := v.values[name]; !seen {
				if isArrayIndex(name) {
					indexes = append(indexes, name)
				} else {
					others = append(others, name)
				}
			}
			v.values[name] = child
		}
		sort.SliceStable(indexes, func(i, j int) bool {
			a, _ := strconv.ParseUint(indexes[i], 10, 32)
			b, _ := strconv.ParseUint(indexes[j], 10, 32)
			return a < b
		})
		v.keys = append(indexes, others...)
		return nil
	case '[':
		return json.Unmarshal(trimmed, &v.items)
	}
	v.scalar = append(json.RawMessage(nil), trimmed...)
	return nil
}

func (v orderedValue) MarshalJSON() ([]byte, error) {
	switch v.kind {
	case '{':
		var out bytes.Buffer
		out.WriteByte('{')
		for i, key := range v.keys {
			if i > 0 {
				out.WriteByte(',')
			}
			name, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}
			value, err := json.Marshal(v.values[key])
			if err != nil {
				return nil, err
			}
			out.Write(name)
			out.WriteByte(':')
			out.Write(value)
		}
		out.WriteByte('}')
		return out.Bytes(), nil
	case '[':
		if v.items == nil {
			return []byte("[]"), nil
		}
		return json.Marshal(v.items)
	}
	return v.scalar, nil
}

// isArrayIndex reports whether a key is one JavaScript orders first: the
// canonical decimal spelling of an integer from 0 to 2^32-2.
func isArrayIndex(key string) bool {
	n, err := strconv.ParseUint(key, 10, 32)
	return err == nil && n < 1<<32-1 && strconv.FormatUint(n, 10) == key
}

package nullplatform

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func findActionSpecByType(specs []*ActionSpecification, actionType string) (*ActionSpecification, error) {
	for _, s := range specs {
		if s.Type == actionType {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no action specification of type %q found", actionType)
}

// projectAttributesToParameters returns a new map containing only the keys
// from `attributes` that are also declared under
// parameterSchema["schema"]["properties"], coercing each value to the JSON
// type the property declares. Defensive: returns an empty map (never nil) on
// missing or malformed schema. Returns an error if any matched value cannot
// be coerced (e.g. "abc" against type=number).
//
// The action specification's `parameters` field has the shape:
//
//	{ "schema": { "type": "object", "properties": {...}, "required": [...] }, "values": {} }
//
// matching the API's serialization of attribute schemas. The actual JSON
// Schema lives one level down at parameters.schema.
//
// Coercion is needed because the resource's `attributes` schema is TypeMap
// with TypeString elem, so all values arrive here as strings — but the
// action's parameter schema declares precise JSON types (number, boolean,
// etc.) and the API will reject mismatches.
func projectAttributesToParameters(attributes map[string]interface{}, parameterSchema map[string]interface{}) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	if parameterSchema == nil || attributes == nil {
		return out, nil
	}
	schemaRaw, ok := parameterSchema["schema"]
	if !ok {
		return out, nil
	}
	schemaMap, ok := schemaRaw.(map[string]interface{})
	if !ok {
		return out, nil
	}
	propsRaw, ok := schemaMap["properties"]
	if !ok {
		return out, nil
	}
	props, ok := propsRaw.(map[string]interface{})
	if !ok {
		return out, nil
	}
	for key, propRaw := range props {
		v, present := attributes[key]
		if !present {
			continue
		}
		propSchema, _ := propRaw.(map[string]interface{})
		coerced, err := coerceToSchemaType(v, propSchema)
		if err != nil {
			return nil, fmt.Errorf("attribute %q: %w", key, err)
		}
		out[key] = coerced
	}
	return out, nil
}

// coerceToSchemaType converts a value to the JSON type declared in
// propertySchema["type"]. If the value is already the right type, it's
// returned unchanged. Strings are parsed for number/integer/boolean/array/
// object types. Unknown or absent types pass through.
func coerceToSchemaType(v interface{}, propertySchema map[string]interface{}) (interface{}, error) {
	if propertySchema == nil {
		return v, nil
	}
	typeStr, _ := propertySchema["type"].(string)
	s, isString := v.(string)
	if !isString {
		return v, nil
	}
	switch typeStr {
	case "string", "":
		return s, nil
	case "number":
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot coerce %q to number: %w", s, err)
		}
		return f, nil
	case "integer":
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot coerce %q to integer: %w", s, err)
		}
		return i, nil
	case "boolean":
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("cannot coerce %q to boolean: %w", s, err)
		}
		return b, nil
	case "array", "object":
		var out interface{}
		if err := json.Unmarshal([]byte(s), &out); err != nil {
			return nil, fmt.Errorf("cannot decode %q as JSON %s: %w", s, typeStr, err)
		}
		return out, nil
	default:
		return v, nil
	}
}

// summarizeMessages returns a short, human-readable string from an action's
// messages list. Prefers the most recent error-severity message; falls back
// to the most recent message of any severity; returns "no message" if none.
func summarizeMessages(messages []interface{}) string {
	if len(messages) == 0 {
		return "no message"
	}
	var lastAny, lastError string
	for _, raw := range messages {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		text, ok := m["message"].(string)
		if !ok {
			continue
		}
		lastAny = text
		if sev, _ := m["severity"].(string); sev == "error" {
			lastError = text
		}
	}
	if lastError != "" {
		return lastError
	}
	if lastAny != "" {
		return lastAny
	}
	return "no message"
}

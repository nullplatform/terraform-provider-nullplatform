package nullplatform

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func serializeHelper(value any) (any, error) {
	rv := reflect.ValueOf(value)

	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return value, nil
	default:
		// Not a basic type, so serialize it
		serialized, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return string(serialized), nil
	}
}

func deserializeHelper(value string) (any, error) {
	rv := reflect.ValueOf(value)

	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return value, nil

	case reflect.String:
		return tryParseJSON(value), nil
	default:
		var formattedValue any
		if err := json.Unmarshal([]byte(value), &formattedValue); err != nil {
			return nil, fmt.Errorf("invalid arguments JSON: %w", err)
		}
		return formattedValue, nil
	}
}

func tryParseJSON(value string) any {
	var parsedValue any
	if err := json.Unmarshal([]byte(value), &parsedValue); err != nil {
		return value
	}
	return parsedValue
}


func pruneNulls(v interface{}) (interface{}, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, vv := range t {
			if pruned, keep := pruneNulls(vv); keep {
				out[k] = pruned
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, vv := range t {
			if pruned, keep := pruneNulls(vv); keep {
				out = append(out, pruned)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	default:
		if t == nil {
			return nil, false
		}
		return t, true
	}
}
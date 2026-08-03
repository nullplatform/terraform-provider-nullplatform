package nullplatform

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// configuredBool reports an Optional+Computed boolean attribute only when the
// practitioner actually declared it in the configuration, returning nil
// otherwise so the caller can leave the field out of the request body and let
// the API keep applying its own default.
//
// d.Get cannot make this distinction: an omitted bool and an explicit false
// both read back as false, so the raw configuration has to be consulted. The
// value itself still comes from d.Get, which is always resolved by the time
// create and update run.
func configuredBool(d *schema.ResourceData, key string) *bool {
	raw := d.GetRawConfig()
	if raw.IsNull() {
		return nil
	}
	if attr := raw.GetAttr(key); attr.IsNull() {
		return nil
	}
	v := d.Get(key).(bool)
	return &v
}

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

func mapOfInterfacesToMapOfStrings(m map[string]interface{}) map[string]string {
	out := make(map[string]string)
	for k, v := range m {
		out[k] = valueToString(v)
	}
	return out
}

func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}

		return "false"
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}

		return fmt.Sprintf("%g", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

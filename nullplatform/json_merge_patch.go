package nullplatform

import (
	"encoding/json"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// mergePatchBetween is the JSON Merge Patch (RFC 7396) that takes `held` to
// `wanted`: a member that changed or is new goes with its value, one that is no
// longer wanted goes as null, and one that did not change does not go. Objects
// are compared member by member; any other value, a list included, goes whole.
//
// A null member of `wanted` is a member that is not there: a merge patch cannot
// store one. A null member of `held` (the platform of before merge patches
// could store one) is a member like any other: not wanted, it goes as null,
// which removes it.
func mergePatchBetween(held, wanted map[string]interface{}) map[string]interface{} {
	wanted = withoutNulls(wanted)
	patch := map[string]interface{}{}

	for name := range held {
		if _, stays := wanted[name]; !stays {
			patch[name] = nil
		}
	}
	for name, value := range wanted {
		heldValue, isHeld := held[name]
		heldObject, heldIsObject := heldValue.(map[string]interface{})
		wantedObject, wantedIsObject := value.(map[string]interface{})

		switch {
		case !isHeld:
			patch[name] = value
		case heldIsObject && wantedIsObject:
			if members := mergePatchBetween(heldObject, wantedObject); len(members) > 0 {
				patch[name] = members
			}
		case !reflect.DeepEqual(heldValue, value):
			patch[name] = value
		}
	}
	return patch
}

// withoutNulls is a copy of `object` with no null member, at any depth of its
// objects. The null of a list is a value of the list and stays.
func withoutNulls(object map[string]interface{}) map[string]interface{} {
	clean := make(map[string]interface{}, len(object))
	for name, value := range object {
		switch value := value.(type) {
		case nil:
		case map[string]interface{}:
			clean[name] = withoutNulls(value)
		default:
			clean[name] = value
		}
	}
	return clean
}

// suppressEquivalentJSONWithoutNulls is suppressEquivalentJSON for an object
// the platform edits as a merge patch: a member written as null is one that is
// left out, so it is no diff against an object that does not have it. What the
// platform holds (`old`) is compared as it is: a null it stores is a diff, and
// the update that follows removes it.
func suppressEquivalentJSONWithoutNulls(_, old, new string, _ *schema.ResourceData) bool {
	var oldObject, newObject map[string]interface{}
	if err := json.Unmarshal([]byte(old), &oldObject); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(new), &newObject); err != nil {
		return false
	}
	return reflect.DeepEqual(oldObject, withoutNulls(newObject))
}

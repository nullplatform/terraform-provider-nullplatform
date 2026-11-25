package nullplatform

import (
	"encoding/json"
	"testing"
)

func TestMergePatchBetween(t *testing.T) {
	cases := []struct {
		name, held, wanted, patch string
	}{
		{"nothing changes", `{"model":"opus","limits":{"tokens":1000}}`, `{"limits":{"tokens":1000},"model":"opus"}`, `{}`},
		{"a value changes", `{"model":"opus","autonomy":"low"}`, `{"model":"fable","autonomy":"low"}`, `{"model":"fable"}`},
		{"a member is added", `{"model":"opus"}`, `{"model":"opus","autonomy":"low"}`, `{"autonomy":"low"}`},
		{"a member is removed", `{"model":"opus","autonomy":"low"}`, `{"model":"opus"}`, `{"autonomy":null}`},
		{"every member is removed", `{"model":"opus","autonomy":"low"}`, `{}`, `{"autonomy":null,"model":null}`},
		{"an object changes member by member", `{"limits":{"tokens":1000,"tools":["search"],"depth":2}}`, `{"limits":{"tokens":2000,"depth":2}}`, `{"limits":{"tokens":2000,"tools":null}}`},
		{"an object is emptied", `{"limits":{"tokens":1000}}`, `{"limits":{}}`, `{"limits":{"tokens":null}}`},
		{"an empty object is added", `{}`, `{"limits":{}}`, `{"limits":{}}`},
		{"a list is replaced whole", `{"tools":["search","code"]}`, `{"tools":["search"]}`, `{"tools":["search"]}`},
		{"an object takes the place of a value", `{"target":"node"}`, `{"target":{"runtime":"deno","flags":null}}`, `{"target":{"runtime":"deno"}}`},
		{"a value takes the place of an object", `{"target":{"runtime":"deno"}}`, `{"target":"node"}`, `{"target":"node"}`},
		{"a null wanted is a member left out", `{"model":"opus","autonomy":"low"}`, `{"model":"opus","autonomy":null,"limits":null}`, `{"autonomy":null}`},
		{"a null held is a member not held", `{"model":"opus","autonomy":null}`, `{"model":"opus"}`, `{}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var held, wanted map[string]interface{}
			if err := json.Unmarshal([]byte(c.held), &held); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(c.wanted), &wanted); err != nil {
				t.Fatal(err)
			}

			patch, err := json.Marshal(mergePatchBetween(held, wanted))
			if err != nil {
				t.Fatal(err)
			}
			if string(patch) != c.patch {
				t.Errorf("from %s to %s: got the patch %s, want %s", c.held, c.wanted, patch, c.patch)
			}
		})
	}
}

func TestSuppressEquivalentJSONWithoutNulls(t *testing.T) {
	cases := []struct {
		name, old, new string
		same           bool
	}{
		{"a null member against none", `{"model":"opus"}`, `{"model":"opus","autonomy":null}`, true},
		{"a null member of an object against none", `{"limits":{"tokens":1}}`, `{"limits":{"tokens":1,"tools":null}}`, true},
		{"another order and spacing", `{"a":1,"b":2}`, `{ "b": 2, "a": 1 }`, true},
		{"another value", `{"model":"opus"}`, `{"model":"fable"}`, false},
		{"a member more", `{"model":"opus"}`, `{"model":"opus","autonomy":"low"}`, false},
		{"a null in a list is a value", `{"tools":["a"]}`, `{"tools":["a",null]}`, false},
		{"what is not JSON", `{"model":"opus"}`, `not json`, false},
		{"what was not JSON", `not json`, `{"model":"opus"}`, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := suppressEquivalentJSONWithoutNulls("attributes", c.old, c.new, nil); got != c.same {
				t.Errorf("%s against %s: suppressed %v, want %v", c.old, c.new, got, c.same)
			}
		})
	}
}

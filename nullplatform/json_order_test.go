package nullplatform

import "testing"

func TestSameJSONInOrder(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"whitespace is not part of the document", `{"a":1,"b":[true,null]}`, "{ \"a\": 1,\n  \"b\": [ true, null ] }", true},
		{"the spelling of a number or a string is not either", `{"a":1.0,"b":"é"}`, `{"a":1,"b":"é"}`, true},
		{"the order of an object's keys is", `{"a":1,"b":2}`, `{"b":2,"a":1}`, false},
		{"the order of nested keys is", `{"p":{"a":1,"b":2}}`, `{"p":{"b":2,"a":1}}`, false},
		{"a different value", `{"a":1}`, `{"a":2}`, false},
		{"not JSON equals nothing", `{"a":`, `{"a":`, false},
		{"nothing equals nothing", ``, ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameJSONInOrder(tc.a, tc.b); got != tc.want {
				t.Errorf("sameJSONInOrder(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			if got := suppressSameJSONInOrder("schema", tc.a, tc.b, nil); got != tc.want {
				t.Errorf("suppressSameJSONInOrder(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestCompactJSON_KeepsOrderAndDropsWhitespace(t *testing.T) {
	got, err := compactJSON([]byte("{ \"b\": 1,\n \"a\": { \"z\": 1, \"y\": 2 } }"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `{"b":1,"a":{"z":1,"y":2}}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, err := compactJSON([]byte(`{"a":`)); err == nil {
		t.Error("want an error for a document that is not JSON")
	}
}

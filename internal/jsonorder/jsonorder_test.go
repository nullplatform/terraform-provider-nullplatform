package jsonorder

import "testing"

func TestJavaScript(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"keys as written", `{ "b": 1, "a": [ {"z": null, "y": true} ] }`, `{"b":1,"a":[{"z":null,"y":true}]}`},
		{"array indexes first, ascending", `{"b":1,"10":2,"a":3,"2":4,"0":5}`, `{"0":5,"2":4,"10":2,"b":1,"a":3}`},
		{"in nested objects too", `{"p":{"x":1,"1":2}}`, `{"p":{"1":2,"x":1}}`},
		{"not canonical, not an index", `{"b":1,"01":2,"-1":3,"1.5":4,"4294967295":5}`, `{"b":1,"01":2,"-1":3,"1.5":4,"4294967295":5}`},
		{"the largest index", `{"b":1,"4294967294":2}`, `{"4294967294":2,"b":1}`},
		{"a key twice keeps its place and its last value", `{"a":1,"b":2,"a":3}`, `{"a":3,"b":2}`},
		{"scalars and empties", `[1,"x",{},[]]`, `[1,"x",{},[]]`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := JavaScript([]byte(c.in))
			if err != nil || string(got) != c.want {
				t.Fatalf("JavaScript(%s) = %s, %v; want %s", c.in, got, err, c.want)
			}
		})
	}

	if _, err := JavaScript([]byte(`{"a":`)); err == nil {
		t.Fatal("want an error for a document that is not JSON")
	}
}

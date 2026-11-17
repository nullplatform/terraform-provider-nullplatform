package nullplatform

import "testing"

func TestSuppressEquivalentJSON(t *testing.T) {
	cases := []struct {
		name string
		old  string // prior state: what the API stored (may hold server-populated defaults)
		new  string // planned config: only what the user set
		want bool   // true => diff suppressed (no change)
	}{
		{
			name: "identical",
			old:  `{"cluster":{"id":"c1"}}`,
			new:  `{"cluster":{"id":"c1"}}`,
			want: true,
		},
		{
			name: "api added sibling keys the config never set",
			old:  `{"cluster":{"id":"c1","namespace":"ns"},"naming":{"strategy":"qualified","scope_pattern":"{.scope.id}","deployment_pattern":"{.deployment.id}"},"traffic_manager":{"version":"v1.9.0","port":80}}`,
			new:  `{"cluster":{"id":"c1","namespace":"ns"},"naming":{"strategy":"qualified"},"traffic_manager":{"version":"v1.9.0"}}`,
			want: true,
		},
		{
			name: "config changes a value it set",
			old:  `{"naming":{"strategy":"qualified","scope_pattern":"{.scope.id}"}}`,
			new:  `{"naming":{"strategy":"custom"}}`,
			want: false,
		},
		{
			name: "config adds a key absent from state",
			old:  `{"cluster":{"id":"c1"}}`,
			new:  `{"cluster":{"id":"c1"},"balancer":{"public_name":"pub"}}`,
			want: false,
		},
		{
			name: "nested value differs",
			old:  `{"cluster":{"id":"c1"},"traffic_manager":{"port":80}}`,
			new:  `{"cluster":{"id":"c1"},"traffic_manager":{"port":10080}}`,
			want: false,
		},
		{
			name: "invalid json is not suppressed",
			old:  `{"cluster":{"id":"c1"}}`,
			new:  `{not json`,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := suppressEquivalentJSON("attributes", tc.old, tc.new, nil); got != tc.want {
				t.Fatalf("suppressEquivalentJSON() = %v, want %v", got, tc.want)
			}
		})
	}
}

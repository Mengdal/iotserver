package deviceapp

import "testing"

func TestPropertyValueString(t *testing.T) {
	cases := []struct {
		name  string
		input interface{}
		want  string
	}{
		{name: "string", input: "on", want: "on"},
		{name: "bool", input: true, want: "true"},
		{name: "int", input: 12, want: "12"},
		{name: "float", input: 12.5, want: "12.5"},
		{name: "map", input: map[string]interface{}{"a": 1}, want: `{"a":1}`},
		{name: "slice", input: []interface{}{1, 2}, want: `[1,2]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := propertyValueString(tc.input)
			if err != nil {
				t.Fatalf("propertyValueString: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	if _, err := propertyValueString(nil); err == nil {
		t.Fatal("nil should fail")
	}
}
